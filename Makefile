work_dir = $(shell pwd)
golangci_version = $(shell head -n 1 .golangci.yml | tr -d '# ')

all: build

agent-image: build-agent test
	docker build -t ghcr.io/grafana/xk6-disruptor-agent:latest images/agent

build: test
	go install go.k6.io/xk6/cmd/xk6@latest
	xk6 build --with $(shell go list -m)=. --output build/k6

build-agent:
	CGO_ENABLED=0 go build -o images/agent/build/xk6-disruptor-agent ./cmd/agent

clean:
	rm -rf image/agent/build build/
	
e2e-api: agent-image
	go test -tags e2e ./e2e/api/...

e2e-cluster:
	go test -tags e2e ./e2e/cluster/...

e2e-http: agent-image
	go test -tags e2e ./e2e/disruptors/http/...

e2e-kubernetes:
	go test -tags e2e ./e2e/kubernetes/...

e2e: e2e-cluster e2e-kubernetes e2e-http e2e-api

format:
	go fmt ./...

lint:
	docker run --rm -v $(work_dir):/disruptor -w /disruptor golangci/golangci-lint:$(golangci_version) golangci-lint run

test:
	go test -race  ./...

.PHONY: agent-image build build-agent clean e2e e2e-api e2e-cluster e2e-http e2e-kubernetes format lint test
