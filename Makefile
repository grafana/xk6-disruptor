all: build-agent

clean:
	rm -rf bin/

build-agent:
	go build  -o bin/xk6-disruptor-agent ./cmd/agent

format:
	go fmt ./...
	
test:
	go test -race  ./...


.PHONY: clean build-agent format test