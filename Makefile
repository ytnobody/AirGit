.PHONY: build run clean test

build:
	go build -o airgit .

run:
	go run .

clean:
	rm -f airgit

test:
	go test -v ./...

# Cross-compile for different platforms
build-linux:
	GOOS=linux GOARCH=amd64 go build -o airgit-linux .

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o airgit-mac .

build-arm:
	GOOS=linux GOARCH=arm64 go build -o airgit-arm64 .
