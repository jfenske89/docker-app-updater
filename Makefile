PROJECT_NAME=docker-app-updater

all: install fmt vet lint test compile

install:
	go mod tidy

fmt:
	go run mvdan.cc/gofumpt@latest -w ./

vet:
	go vet ./...

test:
	go test -v ./...

compile:
	go build -o ./bin/docker-app-updater ./cmd/docker-app-updater

lint:
	golangci-lint run --verbose

upgrade:
	go get -u ./...
	go mod tidy
