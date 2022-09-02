all: agent-image

clean:
	rm -rf bin/

build-agent:
	CGO_ENABLED=0 go build -o images/agent/build/xk6-disruptor-agent ./cmd/agent

agent-image: build-agent test
	docker build -t grafana/xk6-disruptor-agent images/agent

format:
	go fmt ./...
	
test:
	go test -race  ./...

.PHONY: agent-image build-agent clean format test