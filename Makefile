all: agent-image

clean:
	rm -rf image/agent/build

build-agent:
	CGO_ENABLED=0 go build -o images/agent/build/xk6-disruptor-agent ./cmd/agent

agent-image: build-agent test
	docker build -t grafana/xk6-disruptor-agent images/agent

format:
	go fmt ./...
	
test:
	go test -race  ./...

e2e-http: agent-image
	go test -tags e2e ./e2e/disruptors/http/...

e2e-kubernetes:
	go test -tags e2e ./e2e/kubernetes/...

e2e-cluster:
	go test -tags e2e ./e2e/cluster/...

e2e: e2e-cluster e2e-kubernetes e2e-http

.PHONY: agent-image build-agent clean e2e e2e-cluster e2e-http e2e-kubernetes format test
