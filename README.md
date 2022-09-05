# xk6-disruptor

xk6-disruptor is a [k6](https://k6.io) extension for injecting faults into the system under tests.

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

