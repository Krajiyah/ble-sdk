# ble-sdk

[![made-with-go](https://img.shields.io/badge/Made%20with-go-1f425f.svg)](https://www.golang.org/)
[![Build Status](https://secure.travis-ci.org/Krajiyah/ble-sdk.png?branch=master)](http://travis-ci.org/Krajiyah/ble-sdk)
[![Code Coverage](https://img.shields.io/badge/coverage-65%25-green)](http://travis-ci.org/Krajiyah/ble-sdk)
[![GitHub license](https://img.shields.io/github/license/Krajiyah/ble-sdk.svg)](https://github.com/Krajiyah/ble-sdk/blob/master/LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/Krajiyah/ble-sdk.svg)](https://Krajiyah/ble-sdk)

Easy to use Go SDK for BLE service and client. Here is the link to [documentation](https://godoc.org/github.com/Krajiyah/ble-sdk)

## Dependencies for Runtime Environment

### Debian

```bash
sudo apt-get -y update
sudo apt-get -y install bluez
sudo apt-get -y install libglib2.0-dev
sudo apt-get -y install libbluetooth-dev

# May need to run these for client before run (on boot)
sudo hciconfig hci0 up
sudo hciconfig sspmode 1
sudo hciconfig piscan
```

## Examples

- [Client](https://github.com/Krajiyah/ble-sdk/blob/master/examples/client/main.go)
- [Server](https://github.com/Krajiyah/ble-sdk/blob/master/examples/server/main.go)
