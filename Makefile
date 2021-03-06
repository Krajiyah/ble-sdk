BINARY_CLIENT_DST=client.bin
BINARY_FORWARDER_DST=forwarder.bin
BINARY_SERVER_DST=server.bin

all: clean install build

clean:
	go mod tidy
ifneq ("$(wildcard $(BINARY_CLIENT_DST))","")
	rm $(BINARY_CLIENT_DST)
endif
ifneq ("$(wildcard $(BINARY_FORWARDER_DST))","")
	rm $(BINARY_FORWARDER_DST)
endif
ifneq ("$(wildcard $(BINARY_SERVER_DST))","")
	rm $(BINARY_SERVER_DST)
endif

install:
	go mod vendor

build:
ifndef BLESECRET
	$(error BLESECRET must be set)
endif
ifndef BLE_CLIENT_ADDR
	$(error BLE_CLIENT_ADDR must be set)
endif
ifndef BLE_FORWARDER_ADDR
	$(error BLE_FORWARDER_ADDR must be set)
endif
ifndef BLE_SERVER_ADDR
	$(error BLE_SERVER_ADDR must be set)
endif
	 GOOS=linux GOARCH=arm GOARM=5 go build -ldflags "-X main.BLESecret=$(BLESECRET) -X main.BLEClientAddr=$(BLE_CLIENT_ADDR) -X main.BLEServerAddr=$(BLE_SERVER_ADDR)" -o $(BINARY_CLIENT_DST) examples/client/main.go
	 GOOS=linux GOARCH=arm GOARM=5 go build -ldflags "-X main.BLESecret=$(BLESECRET) -X main.BLEForwarderAddr=$(BLE_FORWARDER_ADDR) -X main.BLEServerAddr=$(BLE_SERVER_ADDR)" -o $(BINARY_FORWARDER_DST) examples/forwarder/main.go
	 GOOS=linux GOARCH=arm GOARM=5 go build -ldflags "-X main.BLESecret=$(BLESECRET)" -o $(BINARY_SERVER_DST) examples/server/main.go

test:
	./scripts/test.sh

coverage:
	go tool cover -html=coverage.out