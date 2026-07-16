PROJECT_NAME=docker-app-updater

all: deps fmt lint test compile vuln

deps:
	go mod tidy

deps-update:
	go get -u ./...
	go mod tidy

fmt:
	go run mvdan.cc/gofumpt@latest -w ./
	npx prettier -w *.md --log-level=warn

lint:
	golangci-lint run --verbose

test:
	go test -v ./...

compile:
	go build -o ./bin/docker-app-updater ./cmd/docker-app-updater

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

modernize:
	go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest ./...

modernize-fix:
	go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...
	make test
	make lint

deploy:
	./scripts/deploy.sh
