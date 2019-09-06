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
	go build -o $(BINARY_CLIENT_DST) examples/client/main.go
	go build -o $(BINARY_SERVER_DST) examples/server/main.go

testt:
	go test -v test/*_test.go
