# Nordcloud Klarity Scanner for VMware

This repository contains source code for Klarity's self-hosted scanner of your VMware environments. It's distributed as single self-contained binaries (see Releases) to deploy and schedule in your internal network - so it can access VMware vCenter APIs.

## Prerequisites

- Windows or Linux machines (amd64) with access to vCenter API
- Read-only API credentials for VMware vCenter
- Nordcloud Klarity Cloud Account credentials for VMware

## Other platforms

Binaries for other platforms and architectures are not officialy supported, but you are allowed to compile the scanner with applicable `GOOS` and `GOARCH` variables for Golang compiler and test it.

## Supported versions

Nordcloud Klarity Scanner for VMware should work with vCenter 7

## Installation

1. [Download latest version](https://github.com/nordcloud/klarity-scanner-vmware-cli/releases) of Nordcloud Klarity Scanner for VMware for your platform from Releases tab in GitHub repository.
2. Upload the binary to your virtual machine.
3. Update `config.json` with correct parameters from Nordcloud Klarity and your vCenter environment.
4. Run the binary to check for any issues.
5. Use system scheduler (crontab / Windows Task Scheduler) to run the binary once per day.

## Config description

TODO

## Task scheduler example configuration

TODO
