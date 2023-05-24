# Contributing to xk6-disruptor

This document describes how you can contribute to the xk6-disruptor project.

For proposing significant changes (breaking changes in the API, significant refactoring, implementation of complex features) we suggest creating a [design proposal document](./design-docs/README.md) before starting the implementation, to ensure consensus and avoid reworking on aspects on which there is not agreement.

## Build locally

As contributor, you will need to build locally the disruptor extension with your changes.

### Requirements

Before starting to develop you have [Go](https://golang.org/doc/install), [Git](https://git-scm.com/) and [Docker](https://docs.docker.com/get-docker/) installed. In order to execute use the targets available in the [Makefile](#makefile) you will also need the `make` tool installed. 

### Clone repository

If you you have not already done so, clone this repository to your local machine:
```bash
$ git clone https://github.com/grafana/xk6-disruptor.git
$ cd xk6-disruptor
```

### Makefile

Most of the development tasks can be executed using `make` targets:
* `agent-image`: builds the `xk6-disruptor-agent` image locally
* `build`: builds k6 with the `xk6-disruptor` extension
* `clean`: removes local build and other work directories
* `e2e`: executes the end-to-end tests. These tests can take several minutes.
* `test`: executes unit tests
* `lint`: runs the linter

### Extension/agent image versions dependencies

The xk6-disruptor extension requires an agent image to inject it in its targets.

It is important that the versions of the extension and the agent image are in sync to guarantee compatibility.

When a new release of the disruptor is created by the CI the extension's binary and the agent image are created with matching versions. For example, extension version `v.0.x.0` will use agent image tagged as `v0.x.0`.

Also, an agent image is generated automatically by the CI on each commit to the `main` branch and it is labeled as `latest`.

If you checkout the extension source code and build it locally, it will by default reference the agent image labeled as `latest`.

In this way, an extension built locally from the `main` branch will match the version of the `latest` agent image.

Notice that if you build the agent image locally it will be by default also labeled as `latest`.

### Building the xk6-disruptor-agent image

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

### Debugging the disruptor agent

The disruptor agent is the responsible for injecting faults in the targets (e.g. pods). The agent is injected into the targets by the xk6-disruptor extension as an ephemeral container with the name `xk6-agent`.

You can debug the agent by running it manually at a target.

First enter the agent's container using an interactive console:

```bash
kubectl exec -it <target pod> -c xk6-agent -- sh
```

Once you get the prompt, you can inject faults by running the `xk6-disruptor-agent` command:

```bash
xk6-disruptor-agent [arguments for fault injection]
```

In order to facilitate debugging `xk6-disruptor-agent` offers options for generating execution traces:
* `--trace`: generate traces. The `--trace-file` option allows specifying the output file for traces (default `trace.out`)
* `--cpu-profile`: generate CPU profiling information. The `--cpu-profile-file` option allows specifying the output file for profile information (default `cpu.pprof`)
* `--mem-profile`: generate memory profiling information. The `--mem-profile-file` option allows specifying the output file for profile information (default `mem.pprof`)

In order to analyze those files you have to copy them from the target pod to your local machine. For example, for copying the `trace.out` file:
```bash
kubectl cp <target pod>:trace.out -c xk6-agent trace.out
```

### e2e tests

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

In order to facilitate the development of e2e tests, diverse helper functions have been created, organized in the following packages:

* [pkg/testutils/e2e/cluster](pkg/testutils/e2e/cluster) functions for creating test clusters
* [pkg/testutils/e2e/fixtures](pkg/testutils/e2e/fixtures) functions for creating test resources
* [pkg/testutils/e2e/checks](pkg/testutils/e2e/checks) functions for verifying conditions during the test

We strongly encourage to keep adding reusable functions to these helper packages instead of implementing fixtures and validations for each test, unless strictly necessarily.

Also, you should create the resources for each test in a different namespace to isolate parallel tests and facilitate the teardown. 

The following example shows the structure of a e2e test that creates a cluster and then executes tests using this infrastructure:

```go
//go:build e2e
// +build e2e

package package-to-test

import (
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
)

// Test_E2E function creates the resources for the tests and executes the test functions
func Test_E2E(t *testing.T) {

	// create cluster
	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
		cluster.WithName("e2e-test"),
	)
        if err != nil {
	        t.Errorf("failed to create cluster: %v", err)
	        return
        }

	// delete cluster when all sub-tests end
	t.Cleanup(func() {
		_ = cluster.Delete()
	})

	// get kubernetes client
	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	// Execute test on cluster
	t.Run("Test", func(t *testing.T){
		// create namespace for test
		namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		// delete test resources when test ends
		defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

		// create test resources using k8s fixtures

		// execute test
	})
}
```

### Accessing services from an e2e test

By default, the e2e clusters are configured to run an ingress controller to allow the access to services deployed in the cluster from the e2e tests.

To allow accessing the ingress, the e2e cluster requires a port to be exposed to the host where the test is running. By default his port is `30080`.

This may cause interference between tests that make a test fail with the following message `failed to create cluster: host port is not available 30080`.

If this happens deleting any remaining test cluster and retrying the failed test alone will generally solve this issue.

The port to be exposed by the test cluster can be changed using the `WithIngressPort` option:

```go
	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
		cluster.WithName("e2e-test"),
		cluster.WithIngressPort(30083),
	)
```
