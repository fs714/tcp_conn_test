.PHONY: build

default: build

BUILD_TIME=`date +%FT%T%z`

LDFLAGS=-ldflags "-s -X main.BuildTime=${BUILD_TIME}"

build:
	env GOOS=linux GOARCH=amd64 go build -o bin/tcp_conn_test ${LDFLAGS}
clean:
	rm -rf bin/
