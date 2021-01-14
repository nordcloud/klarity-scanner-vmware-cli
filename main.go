// Copyright 2020 Nordcloud Oy or its affiliates. All Rights Reserved.

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

	errors "github.com/nordcloud/ncerrors/errors"
	log "github.com/sirupsen/logrus"
	"github.com/vmware/govmomi/session/cache"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type report struct {
	Errors         map[string]string        `json:"errors"`
	ScannedObjects map[string][]interface{} `json:"scanned_objects"`
}

type configuration struct {
	VMwareInstalationID  string   `json:"vmware_instalation_id"`
	VMwareAPIURL         string   `json:"vmware_api_url"`
	VMwareAPIUsername    string   `json:"vmware_api_username"`
	VMwareAPIPassword    string   `json:"vmware_api_password"`
	VMwareAPIInsecure    bool     `json:"wmvare_api_insecure"`
	KlarityStorageName   string   `json:"klarity_storage_name"`
	KlarityContainerName string   `json:"klarity_container_name"`
	KlaritySASToken      string   `json:"klarity_sas_token"`
	ScannedObjects       []string `json:"scanned_objects"`
}

type scanner struct {
	Configuration configuration
	VMwareClient  *vim25.Client
	ViewManager   *view.Manager
}

func newVMwareScanner(ctx context.Context) (*scanner, error) {
	configuration, err := readConfiguration()
	if err != nil {
		return nil, err
	}

	vmwareClient, err := getVMwareClient(ctx, configuration)
	if err != nil {
		return nil, err
	}

	return &scanner{
		Configuration: configuration,
		VMwareClient:  vmwareClient,
		ViewManager:   view.NewManager(vmwareClient),
	}, nil
}

func readConfiguration() (c configuration, err error) {
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

	return cfg, nil
}
func getVMwareClient(ctx context.Context, cfg configuration) (*vim25.Client, error) {
	urlFlag := flag.String("url", cfg.VMwareAPIURL, fmt.Sprintf("ESX or vCenter URL [%s]", cfg.VMwareAPIURL))
	insecureFlag := flag.Bool("insecure", cfg.VMwareAPIInsecure, fmt.Sprintf("Don't verify the server's certificate chain [%v]", cfg.VMwareAPIInsecure))

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

	ps := []string{"summary"}
	noSummaryItems := []string{"Folder", "Network"}
	for _, item := range noSummaryItems {
		if item == objectType {
			ps = []string{}
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

	url := fmt.Sprintf(
		"https://%s.blob.core.windows.net/%s/%s/%s.json?%s",
		s.Configuration.KlarityStorageName,
		s.Configuration.KlarityContainerName,
		s.Configuration.VMwareInstalationID,
		time.Now().Format(time.RFC3339),
		s.Configuration.KlaritySASToken,
	)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(file))
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

func execute() error {
	ctx := context.Background()

	scanner, err := newVMwareScanner(ctx)
	if err != nil {
		return errors.WithContext(err, "unable to create VMware scanner", nil)
	}

	so := make(map[string][]interface{})
	e := make(map[string]string)
	for _, objectType := range scanner.Configuration.ScannedObjects {
		so[objectType], err = scanner.scanResources(ctx, objectType)
		if err != nil {
			errors.LogErrorPlain(errors.WithContext(err, fmt.Sprintf("unable to scan object '%s'", objectType), nil))
			e[objectType] = err.Error()
		}
	}

	r := report{
		ScannedObjects: so,
		Errors:         e,
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
