PROJECT_NAME=docker-app-updater

all: install fmt vet lint test compile

install:
	go mod tidy

fmt:
	goimports -w --local codeberg.org/jfenske ./

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
