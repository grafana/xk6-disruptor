# xk6-disruptor

xk6-disruptor is a [k6](https://k6.io) extension for injecting faults into the system under tests.


# Architecture



## xk6-disruptor-agent

The xk6-disruptor-agent is an agent that can inject disruptions in the target system.
It offers a series of commands that inject different types of disruptions described in the
next sections.

### Http

The http command injects disruptions in the requests sent to a target http server.
The target is defined by the tcp port and interface where the target is listening.
The disruptions are defined as either delays in the responses and/or a rate of errors
returned from the request.

The following command shows the options in detail:
```sh
$ xk6-disruptor-agent http -h
Disrupts http request by introducing delays and errors. Requires NET_ADMIM capabilities for setting iptable rules.

Usage:
  xk6-disruptor-agent http [flags]

Flags:
  -a, --average-delay uint     average request delay (milliseconds) (default 100)
  -v, --delay-variation uint   variation in request delay (milliseconds
  -d, --duration duration      duration of the disruptions (default 1m0s)
  -e, --error uint             error code
  -x, --exclude stringArray    path(s) to be excluded from disruption
  -h, --help                   help for http
  -i, --interface string       interface to disrupt (default "eth0")
  -p, --port uint              port the proxy will listen to (default 8080)
  -r, --rate float32           error rate
  -t, --target uint            port the proxy will redirect request to (default 80)

```

# Development

## e2e tests

End to end tests are meant to test the components of the project in a test environment without mocks.
These tests are slow and resource consuming. To prevent them to be executed as part of the `test` target
it is recommended to make their execution conditioned to the `e2e` build tags by adding the following compiler
directives to each test file:

```go
//go:build e2e
// +build e2e
```

The e2e tests are built and executed using the `e2e` target in the Makefile:
```
$ make e2e
```

The following example shows the structure of a e2e test. 

```go
//go:build e2e
// +build e2e

package package-to-test

import (
	"fmt"
	"os"
	"testing"
)

// createResources creates the resources needed by the tests
func createResources() error {
   return nil
}

// TestMain function creates the resources for the tests and executes the test functions
func TestMain(m *testing.M) {

        // create resources
	err := createResources()
	if err != nil {
		fmt.Printf("failed to create resources: %v", err)
		os.Exit(1)
	}

	// run test function(s)
	rc := m.Run()

	// delete resources 

	// terminate with the rc from the tests
	os.Exit(rc)
}

// Test is the test function
func Test(t *testing.T) {

}
```

