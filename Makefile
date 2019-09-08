BINARY_CLIENT_DST=client.bin
BINARY_SERVER_DST=server.bin

all: clean install build

clean:
	go mod tidy
ifneq ("$(wildcard $(BINARY_CLIENT_DST))","")
	rm $(BINARY_CLIENT_DST)
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
	go build -ldflags "-X main.BLESecret=$(BLESECRET)" -o $(BINARY_CLIENT_DST) examples/client/main.go
	go build -ldflags "-X main.BLESecret=$(BLESECRET)" -o $(BINARY_SERVER_DST) examples/server/main.go

test:
	go test -v -cover -coverprofile coverage.out ./pkg/util ./pkg/server ./pkg/models ./pkg/client
coverage:
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out