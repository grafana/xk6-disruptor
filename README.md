# xk6-disruptor

xk6-disruptor is a [k6](https://k6.io) extension for injecting faults into the system under tests.


> ⚠️  xk6-disruptor is in alpha stage. Breaking changes can be introduced at any time without prior notice. USE AT YOUR OWN RISK!

# Usage

## Build binary

The `xk6-disruptor` is a `k6` extension. In order to use it in a `k6` test script, it is necessary to create a custom build of `k6` including the disruptor extension. This can be easily made my downloading this repository and executing the `make build` command:
```bash
$ make build
```

Test scripts can be executed then running the newly created version of `k6` located in the `build` directory:
```bash
$ ./build/k6 run path/to/test/script
```

# API

The `xk6-disruptor` API is organized around disruptors that affect specific targets such as pods or services. These disruptors can inject different types of faults on their targets.

## Faults

### Http Fault

Http faults affect the response received from an http server.

The http faults are described by the following attributes:
- port: port on which the requests will be intercepted
- average_delay: average delay added to requests in milliseconds (default `0ms`)
- delay_variation: variation in the injected delay in milliseconds (default `0ms`)
- error_rate: rate of requests that will return an error, represented as a float in the range `0.0` to `1.0` (default `0.0`)
- error_code: error code to return
- exclude: list of urls to be excluded from disruption (e.g. /health)

## Pod Disruptor

The `PodDisruptor` class allows the injection of different types of faults in pods. The target pod(s) are defined by means of a pod selector.
The faults are injected with the help of a [k6-disruptor-agent](#xk6-disruptor-agent) attached on each of the target pods. The agent is capable of intercepting traffic directed to the pod and apply the desired effect.
 
`constructor`: creates a pod disruptor

    Parameters:
      selector: criteria for selecting the target pod(s).
      options: options for controlling the behavior of the disruptor

The `selector` defines the criteria a pod must satisfy in order to be a valid target:
- namespace: namespace the selector will look for pods
- select: attributes that a pod must match for being selected
- exclude: attributes that if a pod matches, will be excluded (even if matches the select attributes)

The following attributes can be used for selecting or excluding pods:
- `labels`: map with the labels to be matched for selection/exclusion

The `options` control the creation and behavior of the pod disruptor:
- inject_timeout: maximum time for waiting the [agent](#xk6-disruptor-agent) to be ready in the target pods, in seconds (default 30s). Zero value forces default. Negative values force no waiting.

Methods:

`injectHttpFaults`: disrupts http requests served by the target pods.

      Parameters:
        fault: description of the http faults to be injected
        duration: duration of the disruption in seconds (default 30s)
        options: options that control the injection of the fault

The injection of the fault is controlled by the following options:
  - proxy_port: port the agent will use to listen for requests in the target pods ( default `8080`)
  - iface: network interface where the agent will capture the traffic ( default `eth0`)

`targets`: returns the list of target pods for the disruptor.

Example: [`examples/pod_disruptor.js`](examples/pod_disruptor.js) shows how to create a selector that matches all pods in the `default` namespace with the `app=my-app` label and inject a delay of 100ms and a 10% of requests returning a http response code 500. 

```js
import { PodDisruptor } from 'k6/x/disruptor';
  
const selector = {
  namespace: "default"
  select: {
    labels: {
      app: "my-app"
    }
  }
}

const fault = {
        average_delay: 100,
        error_rate: 0.1,
        error_code: 500
}

export default func() {
    const disruptor = new PodDisruptor(selector)
    const targets = disruptor.targets()
    if (targets.length != 1) {
      throw new Error("expected list to have one target")
    }

    disruptor.injectHttpFault(30, fault)
}
```


## Service Disruptor

The `ServiceDisruptor` allows the injection of different types of faults in the pods that back a Kubernetes service.
 
`constructor`: creates a service disruptor

    Parameters:
      service: name of the service
      namespace: namespace on which the service is defined
      options: options for controlling the behavior of the disruptor

The `options` control the creation and behavior of the service disruptor:
- inject_timeout: maximum time for waiting the [agent](#xk6-disruptor-agent) to be ready in the target pods, in seconds (default 30s). Zero value forces default. Negative values force no waiting.

Methods:

`injectHttpFaults`: disrupts http requests served by the target pods.

      Parameters:
        fault: description of the http faults to be injected
        duration: duration of the disruption in seconds (default 30s)
        options: options that control the injection of the fault

The injection of the fault is controlled by the following options:
  - proxy_port: port the agent will use to listen for requests in the target pods ( default `8080`)
  - iface: network interface where the agent will capture the traffic ( default `eth0`)

`targets`: returns the list of target pods for the disruptor.

Example: [`examples/service_disruptor.js`](examples/service_disruptor.js) shows how to create a disruptor for a service and inject a delay of 100ms and a 10% of requests returning a http response code 500. 

```js
import { ServiceDisruptor } from 'k6/x/disruptor';
  

const fault = {
        average_delay: 100,
        error_rate: 0.1,
        error_code: 500
}

export default func() {
    const disruptor = new ServiceDisruptor("service", "default")
    const targets = disruptor.targets()
    if (targets.length != 1) {
      throw new Error("expected list to have one target")
    }

    disruptor.injectHttpFault(30, fault)
}
```


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

In order to facilitate the development of e2e tests, some helper functions have been created in the `pkg/testutils/e2e/fixtures` package for creating test resources, including a test cluster, and in `pkg/testutils/e2e/checks` package for verifying conditions during the test. We strongly encourage to keep adding reusable functions to these helper packages instead of implementing fixtures and validations for each test, unless strictly necessarily. 

The following example shows the structure of a e2e test that creates a cluster and then executes tests using this infrastructure

```go
//go:build e2e
// +build e2e

package package-to-test

import (
	"os"
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"



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
