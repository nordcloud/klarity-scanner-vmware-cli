// Copyright 2020-2021 Nordcloud Oy or its affiliates. All Rights Reserved.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-playground/validator"
	errors "github.com/nordcloud/ncerrors/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type tagMapping struct {
	ResType  string `json:"res_type"`
	ResValue string `json:"res_value"`
	TagName  string `json:"tag_name"`
	TagValue string `json:"tag_value"`
}

type report struct {
	Errors         map[string]string        `json:"errors"`
	TagsMapping    []tagMapping             `json:"tags_mapping"`
	ScannedObjects map[string][]interface{} `json:"scanned_objects"`
}

type configuration struct {
	VMwareAPIURL          string   `json:"vmware_api_url" validate:"required"`
	VMwareAPIUsername     string   `json:"vmware_api_username" validate:"required"`
	VMwareAPIPassword     string   `json:"vmware_api_password" validate:"required"`
	VMwareAPIInsecure     bool     `json:"wmvare_api_insecure" validate:"required"`
	KlarityCustomerID     string   `json:"klarity_customer_id" validate:"required"`
	KlarityInstallationID string   `json:"klarity_installation_id" validate:"required"`
	KlarityStorageName    string   `json:"klarity_storage_name" validate:"required"`
	KlaritySASToken       string   `json:"klarity_sas_token" validate:"required"`
	ScannedObjects        []string `json:"scanned_objects" validate:"required"`
}

type scanner struct {
	Configuration configuration
	VMwareClient  *vim25.Client
	ViewManager   *view.Manager
}

func newVMwareScanner(ctx context.Context) (*scanner, error) {
	c, err := readConfiguration()
	if err != nil {
		return nil, err
	}

	vmwareClient, err := getVMwareClient(ctx, c)
	if err != nil {
		return nil, err
	}

	return &scanner{
		Configuration: c,
		VMwareClient:  vmwareClient,
		ViewManager:   view.NewManager(vmwareClient),
	}, nil
}

func readConfiguration() (c configuration, err error) {
	validate := validator.New()

	jsonFile, err := os.Open("config.json")
	if err != nil {
		return c, errors.WithContext(err, "cannot load `config.json` file", nil)
	}
	defer jsonFile.Close()

	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return c, errors.WithContext(err, "cannot load `config.json` file", nil)
	}

	var cfg configuration
	if err := json.Unmarshal(byteValue, &cfg); err != nil {
		return c, errors.WithContext(err, "bad configuration", nil)
	}

	if err := validate.Struct(cfg); err != nil {
		return c, errors.WithContext(err, "bad configuration", nil)
	}

	return cfg, nil
}

func getVMwareClient(ctx context.Context, cfg configuration) (*vim25.Client, error) {
	urlFlag := flag.String("url", cfg.VMwareAPIURL, fmt.Sprintf("ESX or vCenter URL [%s]", cfg.VMwareAPIURL))
	insecureFlag := flag.Bool(
		"insecure",
		cfg.VMwareAPIInsecure,
		fmt.Sprintf("Don't verify the server's certificate chain [%v]", cfg.VMwareAPIInsecure))

	// Parse URL from string
	u, err := soap.ParseURL(*urlFlag)
	if err != nil {
		return nil, err
	}

	u.User = url.UserPassword(cfg.VMwareAPIUsername, cfg.VMwareAPIPassword)

	// Share govc's session cache
	s := &cache.Session{
		URL:      u,
		Insecure: *insecureFlag,
	}

	c := new(vim25.Client)
	err = s.Login(ctx, c, nil)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s scanner) scanResources(ctx context.Context, objectType string) ([]interface{}, error) {
	v, err := s.ViewManager.CreateContainerView(ctx, s.VMwareClient.ServiceContent.RootFolder, []string{objectType}, true)
	if err != nil {
		return nil, errors.WithContext(err, "unable to create cointaner viewer", nil)
	}
	defer v.Destroy(ctx) //nolint:errcheck

	ps := []string{"name", "tag", "summary"}
	noSummaryItems := []string{"Folder", "Network"}
	for _, item := range noSummaryItems {
		if item == objectType {
			ps = []string{"name", "tag"}
		}
	}

	var resources []interface{}
	err = v.Retrieve(ctx, []string{objectType}, ps, &resources)
	if err != nil {
		return nil, errors.WithContext(err, "unable to retrieve object", nil)
	}

	return resources, nil
}

func (s scanner) saveReport(r report) error {
	file, err := json.Marshal(r)
	if err != nil {
		return errors.WithContext(err, "bad data in report", nil)
	}

	reportURL := fmt.Sprintf(
		"https://%s.blob.core.windows.net/%s/%s/%s.json?%s",
		s.Configuration.KlarityStorageName,
		s.Configuration.KlarityCustomerID,
		s.Configuration.KlarityInstallationID,
		time.Now().Format(time.RFC3339),
		s.Configuration.KlaritySASToken,
	)
	req, err := http.NewRequest("PUT", reportURL, bytes.NewReader(file))
	if err != nil {
		return errors.WithContext(err, "cannot create HTTP request", nil)
	}
	req.Header.Set("x-ms-blob-type", "BlockBlob")
	req.Header.Set("x-ms-date", time.Now().String())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return errors.WithContext(err, "cannot upload report to Klarity", nil)
	}
	defer res.Body.Close()

	return nil
}

func getTagsMapping(ctx context.Context, scanner *scanner) []tagMapping {
	re := rest.NewClient(scanner.VMwareClient)
	if err := re.Login(ctx, url.UserPassword(
		scanner.Configuration.VMwareAPIUsername,
		scanner.Configuration.VMwareAPIPassword,
	)); err != nil {
		errors.LogErrorPlain(errors.WithContext(err, "unable to login to REST API", nil))
		return nil
	}

	m := tags.NewManager(re)
	allTags, err := m.GetTags(ctx)
	if err != nil {
		errors.LogErrorPlain(errors.WithContext(err, "unable to get tags", nil))
		return nil
	}

	tagsMapping := []tagMapping{}
	for _, tag := range allTags {
		obj, err := m.GetAttachedObjectsOnTags(ctx, []string{tag.Name})
		if err != nil {
			errors.LogErrorPlain(errors.WithContext(err, "unable to get attached objects on tags", nil))
			continue
		}

		if len(obj) == 0 {
			continue
		}

		category, err := m.GetCategory(ctx, tag.CategoryID)
		if err != nil {
			errors.LogErrorPlain(errors.WithContext(err, "unable to get category", nil))
			continue
		}

		// we search for only one tag, so there is only one object
		for _, elem := range obj[0].ObjectIDs {
			tagsMapping = append(tagsMapping, tagMapping{
				ResType:  elem.Reference().Type,
				ResValue: elem.Reference().Value,
				TagName:  category.Name,
				TagValue: tag.Name,
			})
		}
	}

	return tagsMapping
}

func execute() error {
	ctx := context.Background()

	scanner, err := newVMwareScanner(ctx)
	if err != nil {
		return errors.WithContext(err, "unable to create VMware scanner", nil)
	}

	so := make(map[string][]interface{})
	errs := make(map[string]string)
	for _, objectType := range scanner.Configuration.ScannedObjects {
		so[objectType], err = scanner.scanResources(ctx, objectType)
		if err != nil {
			errors.LogErrorPlain(errors.WithContext(err, fmt.Sprintf("unable to scan object '%s'", objectType), nil))
			errs[objectType] = err.Error()
		}
	}

	r := report{
		ScannedObjects: so,
		TagsMapping:    getTagsMapping(ctx, scanner),
		Errors:         errs,
	}

	return scanner.saveReport(r)
}

func main() {
	log.SetFormatter(&log.TextFormatter{})
	log.SetOutput(os.Stderr)
	log.SetLevel(log.WarnLevel)

	if err := execute(); err != nil {
		errors.LogErrorPlain(err)
	}
}
