work_dir = $(shell pwd)
golangci_version = $(shell head -n 1 .golangci.yml | tr -d '\# ')
arch = $(shell go env GOARCH)
image ?= ghcr.io/grafana/xk6-disruptor:latest
agent_image ?= ghcr.io/grafana/xk6-disruptor-agent:latest

all: build

agent-image: build-agent
	docker build --build-arg TARGETARCH=${arch} -t $(agent_image) images/agent

disruptor-image:
	./build-package.sh -o linux -a ${arch} -v latest -b image/dist/build build
	docker build --build-arg TARGETARCH=${arch} -t $(image) images/disruptor

build: test
	go install go.k6.io/xk6/cmd/xk6@latest
	xk6 build --with $(shell go list -m)=. --output build/k6

build-e2e:
	go build -tags e2e -o build/e2e-cluster ./cmd/e2e-cluster/main.go

build-agent:
	go test ./pkg/agent/...
	GOOS=linux CGO_ENABLED=0 go build -o images/agent/build/xk6-disruptor-agent-linux-${arch} ./cmd/agent

clean:
	rm -rf image/agent/build build/
	
e2e-disruptors: agent-image e2e-setup
	E2E_REUSE=1 go test -tags e2e ./e2e/disruptors/...

e2e-cluster:
	go test -tags e2e ./e2e/cluster/...

e2e-kubernetes:
	go test -tags e2e ./e2e/kubernetes/...

e2e-setup: build-e2e
	build/e2e-cluster setup

format:
	go fmt ./...

integration-agent: agent-image
	go test -tags integration ./pkg/agent/...

integration-kubernetes:
	go test -tags integration ./pkg/kubernetes/...

integration: integration-agent integration-kubernetes

# Running with -buildvcs=false works around the issue of `go list all` failing when git, which runs as root inside
# the container, refuses to operate on the disruptor source tree as it is not owned by the same user (root).
lint:
	docker run --rm -v $(work_dir):/disruptor -w /disruptor -e GOFLAGS=-buildvcs=false golangci/golangci-lint:$(golangci_version) golangci-lint run

test:
	go test -race  ./...

