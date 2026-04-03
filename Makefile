.PHONY: build test lint clean generate

build:
	go build ./...

test:
	go test ./... -v

lint:
	golangci-lint run ./...

clean:
	rm -f bin/karpenter-provider-nirvana

generate:
	controller-gen object paths=./pkg/apis/...

docker-build:
	docker build -t karpenter-provider-nirvana:latest .
