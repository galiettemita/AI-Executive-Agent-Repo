.PHONY: build test lint migrate docker-build

build:
	go build ./...

test:
	go test ./... -count=1

lint:
	gofmt -w .
	go vet ./...

migrate:
	@echo "Run database migrations using your migration runner"

docker-build:
	docker build -t brevio:local .
