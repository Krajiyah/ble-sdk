BINARY_CLIENT_DST=examples/client
BINARY_SERVER_DST=examples/server

all: clean install build
clean:
  go mod tidy
  [ -e $(BINARY_CLIENT_DST) ] && rm $(BINARY_CLIENT_DST)
  [ -e $(BINARY_SERVER_DST) ] && rm $(BINARY_SERVER_DST)
install:
  go mod vendor
build:
	go build -o $(BINARY_CLIENT_DST) examples/client.go
  go build -o $(BINARY_SERVER_DST) examples/server.go
