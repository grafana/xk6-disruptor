# Contributing to xk6-disruptor

This section is for users that would like to contribute to the xk6-disruptor project.

## Requirements

Before starting to develop you have [Go](https://golang.org/doc/install), [Git](https://git-scm.com/) and [Docker](https://docs.docker.com/get-docker/) installed. In order to execute use the targets available in the [Makefile](#makefile) you will also need the `make` tool installed. 

## Clone repository

If you you have not already done so, clone this repository to your local machine:
```bash
$ git clone https://github.com/grafana/xk6-disruptor.git
$ cd xk6-disruptor
```

## Makefile

Most of the development tasks can be executed using `make` targets:
* `agent-image`: builds the `xk6-disruptor-agent` image locally
* `build`: builds k6 with the `xk6-disruptor` extension
* `clean`: removes local build and other work directories
* `e2e`: executes the end-to-end tests. These tests can take several minutes.
* `test`: executes unit tests

## Extension/agent image versions dependencies

The xk6-disruptor extension requires an agent image to inject it in its targets.

It is important that the versions of the extension and the agent image are in sync to guarantee compatibility.

When a new release of the disruptor is created by the CI the extension's binary and the agent image are created with matching versions. For example, extension version `v.0.x.0` will use agent image tagged as `v0.x.0`.

Also, an agent image is generated automatically by the CI on each commit to the `main` branch and it is labeled as `latest`.

If you checkout the extension source code and build it locally, it will by default reference the agent image labeled as `latest`.

In this way, an extension built locally from the `main` branch will match the version of the `latest` agent image.

Notice that if you build the agent image locally it will be by default also labeled as `latest`.

## Building the xk6-disruptor-agent image

If you modify the [xk6-disruptor-agent](./02-architecture.md#xk6-disruptor-agent) you have to build the image and made it available in the test environment.

For building the image use the following command:

```bash
$ make agent-image
```

Once the image is built, how to make it available to the Kubernetes cluster depends on the platform you use for testing.

If you are using a local cluster for your tests such as [Kind](https://kind.sigs.k8s.io/) or [Minikube](https://github.com/kubernetes/minikube) you can make the image available by loading it into the test cluster. 

If using `kind` the following command loads the image into the cluster

```
kind load docker-image ghcr.io/grafana/xk6-disruptor-agent:latest
```

If using `minikube` the following command loads the image into the cluster:

```bash
minikube image load ghcr.io/grafana/xk6-disruptor-agent:latest
```

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

In order to facilitate the development of e2e tests, some helper functions have been created in the [pkg/testutils/e2e/fixtures](pkg/testutils/e2e/fixtures) package for creating test resources, including a test cluster, and in [pkg/testutils/e2e/checks](pkg/testutils/e2e/checks) package for verifying conditions during the test. We strongly encourage to keep adding reusable functions to these helper packages instead of implementing fixtures and validations for each test, unless strictly necessarily.

The following example shows the structure of a e2e test that creates a cluster and then executes tests using this infrastructure:

```go
//go:build e2e
// +build e2e

package package-to-test

import (

	"testing"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
)

// Test_E2E function creates the resources for the tests and executes the test functions
func Test_E2E(t *testing.T) {

        // create cluster
        cluster, err := fixtures.BuildCluster("e2e-test")
        if err != nil {
	        t.Errorf("failed to create cluster: %v", err)
	        return
        }
	      defer cluster.Delete()

	      // Execute test on resources
	      t.Run("Test", func(t *testing.T){
                // execute test
	      })
}
```
> Some e2e tests require ports exposed from the test cluster to the host where the test is running. This may cause interference between tests that make a test fail with the following message `failed to create cluster: host port is not available 32080`. If this happens deleting any remaining test cluster and retrying the failed test alone will generally solve this issue.
