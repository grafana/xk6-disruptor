# xk6-disruptor

The xk6-disruptor is a [k6](https://k6.io) extension providing fault injection capabilities to test system's reliability under turbulent conditions. Think of it as "like unit testing, for reliability". 

This project The project aims to aid developers in building reliable systems in k8s, implementing the goals of "Chaos Engineering" discipline in a k6 way - with the best developer experience as its primary objective. 

xk6-disruptor is intended for systems running in kubernetes. Other platforms are not supported at this time.

The extension offers an API for creating disruptors that target one specific type of component (for example, Pods) and are capable of injecting different types of faults such as errors in HTTP requests served by that component. Currently disruptors exist for [Pods](#pod-disruptor] and [Services](#service-disruptor), but others will be introduced in the future as well as additional types of faults for the existing disruptors.

> ⚠️  xk6-disruptor is in the alpha stage, undergoing active development. We do not guarantee API compatibility between releases - your k6 scripts may need to be updated on each release until this extension reaches v1.0 release.

# Get started

## Requirements

The `xk6-disruptor` is a `k6` extension. In order to use it in a `k6` test script, it is necessary to use a custom build of `k6` that includes the disruptor. See the [Installation](#installation) section below for instructions on how to create this custom build.

The `xk6-disruptor` needs to interact with the Kubernetes cluster on which the application under test is running. In order to do so, you must have the credentials to access the cluster in a [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file. Ensure this file is pointed by the `KUBECONFIG` environment variable or it is located at the default location `$HOME/.kube/config`.

`xk6-disruptor` requires the [grafana/xk6-disruptor-agent](https://github.com/grafana/xk6-disruptor/pkgs/container/xk6-disruptor-agent) image for injecting the [disruptor agent](#xk6-disruptor-agent) into the disruption targets. Kubernetes clusters can be configured to restrict the download of images from public repositories. You need to ensure this image is available in the cluster where the application under test is running. Additionally, the xk6-disruptor-agent must run with network access privileges. Kubernetes clusters [can be configured to restrict the privileges of containers](https://kubernetes.io/docs/concepts/security/pod-security-admission/). If you find an error similar to the one shown below when using the xk6-disruptor, contact your cluster administrator and request the necessary privileges.

> ERROR[0000] error creating PodDisruptor: pods "nginx" is forbidden: violates PodSecurity "baseline:latest": non-default capabilities (container "xk6-agent" must not include "NET_ADMIN" in securityContext.capabilities.add)


You also need to ensure your test application is accessible from the machine where the tests run. See the [Exposing your application](#exposing-your-application) section for instructions on how to make your application available from outside the Kubernetes cluster.

## Installation

### Built from source

Before building a custom `k6` image that contains the `xk6-disruptor` extension ensure you have [Go 1.18](https://golang.org/doc/install) and [Git](https://git-scm.com/) installed.

Once these requirements are satisfied, you will also need to install the [xk6 build tool](https://github.com/grafana/xk6#command-usage):
```bash
$ go install go.k6.io/xk6/cmd/xk6@latest
```

Then you will need to clone the source code from the `k6s-disruptor` repository:
```bash
$ git clone https://github.com/grafana/xk6-disruptor.git
$ cd xk6-disruptor
```

The custom binary can be then built my executing the following command:
```bash
$ xk6 build --with github.com/grafana/xk6-disruptor=. --with github.com/grafana/xk6-kubernetes --output build/k6
```

Notice that we are including both the `xk6-disruptor` and the [xk6-kubernetes extension](https://github.com/grafana/xk6-kubernetes) when building the custom `k6` binary using the command above. This is because many example scripts use the `xk6-kubernetes` extension for creating the Kubernetes resources they need, such as Pods and Services. If you don't use this extension in your tests you can build the custom `k6` binary with only the `xks-disruptor` extension using the following command instead:
```bash
$ xk6 build --with github.com/grafana/xk6-disruptor=. --output build/k6
```

The test scripts can be executed then running the newly created version of `k6` located in the `build` directory:
```bash
$ ./build/k6 run path/to/test/script
```

## Exposing your application

In order to access your application from the test scripts, it must be assigned an external IP in the cluster it is running at. This can be accomplished in different ways depending on the platform you used for deploying the application.

### As a LoadBalancer service
A service of type [`LoadBalancer`](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/) receives an external IP from an external load balancer provider. The load balancer is configured in different ways depending on the platform your cluster is deployed at and the configuration of the cluster. In the following sections we provide guidelines for exposing your application when running in common development environments. If your cluster is deployed in a public cloud, check your cloud provider's documentation.

#### Configuring a LoadBalancer in Kind
[Kind](https://kind.sigs.k8s.io/) is a tool for running local Kubernetes clusters using Docker container to emulate “nodes”. It may be used for local development or CI. Services deployed in a kind cluster can be exposed to be accessed from the host machine [using metallb as a load balancer](https://kind.sigs.k8s.io/docs/user/loadbalancer).

#### Configuring a LoadBalancer in Minikube

[Minikube](https://github.com/kubernetes/minikube) implements a local Kubernetes cluster supporting different technologies for virtualizing the cluster's infrastructure, such as containers, VMs or running in bare metal.

Minikube's tunnel command runs as a process, creating a network route on the host to the service CIDR of the cluster using the cluster’s IP address as a gateway. The tunnel command exposes the external IP directly to any program running on the host operating system.

```console
$ minikube tunnel
```

# API Reference

The `xk6-disruptor` API is organized around disruptors that affect specific targets such as pods or services. These disruptors can inject different types of faults on their targets.


## Pod Disruptor

The `PodDisruptor` class allows the injection of different types of faults in pods. The target pod(s) are defined by means of a pod selector.
The faults are injected with the help of a [k6-disruptor-agent](#xk6-disruptor-agent) attached on each of the target pods. The agent is capable of intercepting traffic directed to the pod and apply the desired effect.
 
`constructor`: creates a pod disruptor

Parameters:
- selector: criteria for selecting the target pods.
- options: options for controlling the behavior of the disruptor

The `selector` defines the criteria a pod must satisfy in order to be a valid target:
- namespace: namespace the selector will look for pods
- select: attributes that a pod must match for being selected
- exclude: attributes that if a pod matches, will be excluded (even if it matches the select attributes)

The following attributes can be used for selecting or excluding pods:
- `labels`: map with the labels to be matched for selection/exclusion

The `options` control the creation and behavior of the pod disruptor:
- injectTimeout: maximum time for waiting the [agent](#xk6-disruptor-agent) to be ready in the target pods, in seconds (default 30s). Zero value forces default. Negative values force no waiting.


`injectHTTPFaults`: disrupts http requests served by the target pods.

Parameters:

- fault: description of the http faults to be injected
- duration: duration of the disruption in seconds
- options: options that control the injection of the fault

The http faults are described by the following attributes:
- port: port on which the requests will be intercepted
- averageDelay: average delay added to requests in milliseconds (default `0ms`)
- delayVariation: variation in the injected delay in milliseconds (default `0ms`)
- errorRate: rate of requests that will return an error, represented as a float in the range `0.0` to `1.0` (default `0.0`)
- errorCode: error code to return
- exclude: list of urls to be excluded from disruption (e.g. /health)

The injection of the fault is controlled by the following options:
  - proxyPort: port the agent will use to listen for requests in the target pods ( default `8080`)
  - iface: network interface where the agent will capture the traffic ( default `eth0`)

`targets`: returns the list of target pods for the disruptor.

Example: [`examples/pod_disruptor.js`](examples/pod_disruptor.js) shows how to create a selector that matches all pods in the `default` namespace with the `run=nginx` label and inject a delay of 100ms and a 10% of requests returning a http response code 500.


```js
import { PodDisruptor } from 'k6/x/disruptor';

const selector = {
        namespace: "default",
        select: {
                labels: {
                        run: "nginx"
                }
        }
}

const fault = {
        averageDelay: 100,
        errorRate: 0.1,
        errorCode: 500
}

export default function () {
        const disruptor = new PodDisruptor(selector)
        const targets = disruptor.targets()
        if (targets.length != 1) {
        	throw new Error("expected list to have one target")
        }

       disruptor.injectHTTPFaults(fault, 30)
}

```

You can test this script by creating first a pod running nginx with the command below, assuming you have [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) installed in your environment:
```bash
> kubectl run nginx --image=nginx
```

You can also use the [xk6-kubernetes](https://github.com/grafana/xk6-kubernetes) extension for creating these resources from your test script.

## Service Disruptor

The `ServiceDisruptor` class allows the injection of different types of faults in the pods that back a Kubernetes service. The disruptor uses the selector attribute in the service definition for selecting the target pods.
 
`constructor`: creates a service disruptor

Parameters:
- service: name of the service
- namespace: namespace on which the service is defined
- options: options for controlling the behavior of the disruptor

The `options` control the creation and behavior of the service disruptor:
- injectTimeout: maximum time for waiting the [agent](#xk6-disruptor-agent) to be ready in the target pods, in seconds (default 30s). Zero value forces default. Negative values force no waiting.


`injectHTTPFaults`: disrupts http requests served by the target pods.

Parameters:
- fault: description of the http faults to be injected
- duration: duration of the disruption in seconds (default 30s)
- options: options that control the injection of the fault

The http faults are described by the following attributes:
- port: port on which the requests will be intercepted
- averageDelay: average delay added to requests in milliseconds (default `0ms`)
- delayVariation: variation in the injected delay in milliseconds (default `0ms`)
- errorRate: rate of requests that will return an error, represented as a float in the range `0.0` to `1.0` (default `0.0`)
- errorCode: error code to return
- exclude: list of urls to be excluded from disruption (e.g. /health)

The injection of the fault is controlled by the following options:
  - proxyPort: port the agent will use to listen for requests in the target pods ( default `8080`)
  - iface: network interface where the agent will capture the traffic ( default `eth0`)

`targets`: returns the list of target pods for the disruptor.

Example: [`examples/service_disruptor.js`](examples/service_disruptor.js) shows how to create a disruptor for the `nginx` service and inject a delay of 100ms and a 10% of requests returning a http response code 500. 

```js
import { ServiceDisruptor } from 'k6/x/disruptor';
  

const fault = {
        averageDelay: 100,
        errorRate: 0.1,
        errorCode: 500
}

export default function() {
    const disruptor = new ServiceDisruptor("nginx", "default")
    const targets = disruptor.targets()
    if (targets.length != 1) {
      throw new Error("expected list to have one target")
    }

    disruptor.injectHTTPFaults(fault, 30)
}
```

You can test this script by creating first a pod running nginx and exposing it as a service with the commands below, assuming you have [kubectl](https://kubernetes.io/docs/tasks/tools/#kubectl) installed in your environment:
```bash
> kubectl run nginx --image=nginx
> kubectl expose pod nginx --port 80
```

You can also use the [xk6-kubernetes](https://github.com/grafana/xk6-kubernetes) extension for creating these resources from your test script.

# Examples

In the next sections we present some examples of how to use the `xk6-disruptor` extension to introduce faults in `k6` tests. Some examples make use the [k6-kubernetes](http://github.com/grafana/xk6-kubernetes) extension. Ensure the custom `k6` binary you are using is built with both `xk6-disruptor` and `xk6-kubernetes` extensions. See the [Installation guide](#installation) for details.

Also, check your test environment satisfies the requirements described in the [get started guide](#requirements). In particular, check you have the credentials for accessing the Kubernetes cluster used for the test properly configured and that this cluster is configured for exposing your application using an external IP. 

## Introduce disruptions in http request to a pod

The example at [examples/httpbin/disrupt-pod.js](examples/httpbin/disrupt-pod.js) shows how `PodDisruptor` can be used for testing the effect of disruptions in the HTTP requests served by a pod. The example deploys a pod running the [httpbin](https://httpbin.org), a simple request/response application that offers endpoints for testing different HTTP request. The test consists in two load generating scenarios: one for obtaining baseline results and another for checking the effect of the faults introduced by the `PodDisruptor`, and one additional scenario for injecting the faults.

Next sections examine the sample code below in detail, describing the different steps in the test life-cycle.

### Initialization

The initialization code imports the external dependencies required by the test. The `Kubernetes` class imported from the `xk6-kubernetes` extension (line 1) provides functions for handling Kubernetes resources. The `PodDisruptor` class imported from the `xk6-disruptor` extension (line 2) provides functions for injecting faults in pods. The [k6/http](https://k6.io/docs/javascript-api/k6-http/) module (line 3) provides functions for executing HTTP requests. 

The built-in [open](https://k6.io/docs/javascript-api/init-context/open) function is used for reading the YAML manifests of the Kubernetes resources needed by test (lines 6-8). 


Finally some constants are defined: the name of the pod and service running the `httpbin` application (line 10), the namespace on which the application is running (line 11) and the timeout used for waiting the resources to be ready (line 12).

```javascript
  1 import { Kubernetes } from 'k6/x/kubernetes';
  2 import { PodDisruptor } from 'k6/x/disruptor';
  3 import  http from 'k6/http';
  4
  5 // read manifests for resources used in the test
  6 const podManifest = open("./manifests/pod.yaml")
  7 const svcManifest = open("./manifests/service.yaml")
  8 const nsManifest  = open("./manifests/namespace.yaml")
  9
 10 const app = "httpbin"
 11 const namespace = "httpbin-ns"
 12 const timeout = 10
 ```

### Setup and teardown

The `setup` function creates the Kubernetes resources needed by the test using the `apply` function provided by the `Kubernetes` class. The resources are defined as `yaml` manifests imported in the init code. It creates a namespace (line 18) for isolating the test from other tests running in the same cluster, then deploys the application as a pod (line 21) and waits until the pod is ready using the helper function `waitPodRunning` (line 22). The pod is exposed as a service (line 28) and the `getExternalIP` function is used for waiting until the service is assigned an IP for being accessed from outside the cluster (line 29). This IP address is then returned as part of the setup data to be used by the test code (line 35-37).

```javascript
 14 export function setup() {
 15     const k8s = new Kubernetes()
 16
 17    // create namespace for isolating test
 18    k8s.apply(nsManifest)
 19
 20    // create a test deployment and wait until is ready
 21    k8s.apply(podManifest)
 22    const ready = k8s.helpers(namespace).waitPodRunning(app, timeout)
 23    if (!ready) {
 24        throw "aborting test. Pod "+ app + " not ready after " + timeout + " seconds"
 25    }
 26
 27    // expose deployment as a service
 28    k8s.apply(svcManifest)
 29    const ip = k8s.helpers(namespace).getExternalIP(app, timeout)
 30    if (ip == "") {
 31        throw "aborting test. Service " + app + " have no external IP after " + timeout + " seconds"
 32    }
 33
 34    // pass service ip to scenarios
 35    return {
 36        srv_ip: ip,
 37    }
 38 }
 ```

The `teardown` function is invoked when the test ends to cleanup all resources. As all the resources created by the tests are defined in a namespace, the teardown logic only has to delete this namespace and all associated resources will be deleted (line 42).

```javascript
 40 export function teardown(data) {
 41    const k8s = new Kubernetes()
 42    k8s.delete("Namespace", namespace)
 43 }
 ```

### Test Load

The test load is generated by the `default` function, which executes a request to the `httpbin` service using the IP address obtained int the `setup` function. The test makes requests to the endpoint `delay/0.1` which will return after `0.1` seconds (`100ms`).

```javascript
 45 export default function(data) {
 46     http.get(`http://${data.srv_ip}/delay/0.1`);
 47 }
 ```

 > The test uses the `delay` endpoint which return after the requested delay. It requests a `0.1s` (`100ms`) delay to ensure the baseline scenario (see scenarios below) has meaningful statistics for the request duration. If we were simply calling a locally deployed http server (for example `nginx`), the response time would exhibit a large variation between a few microseconds to a few milliseconds. Having `100ms` as baseline response time has proved to offer more consistent results.

### Fault injection

The `disrupt` function creates a [PodDisruptor](#pod-disruptor) using a selector that matches pods in the namespace `httpbin-ns` with the label `app: httpbin` (lines 50-58). 

The http faults are then injected by calling the `PodDisruptor`'s `injectHTTPFaults` method using a [fault definition](#http-fault) that introduces a delay of `50ms` on each request and an error code `500` in a `10%` of the requests (lines 61-65).

```javascript
 49 export function disrupt(data) {
 50     const selector = {
 51         namespace: namespace,
 52             select: {
 53                 labels: {
 54                     app: app
 55                 }
 56         }
 57     }
 58     const podDisruptor = new PodDisruptor(selector)
 59  
 60     // delay traffic from one random replica of the deployment
 61     const fault = {
 62         averageDelay: 50,
 63         errorCode: 500,
 64         errorRate: 0.1
 65     }
 66     podDisruptor.injectHTTPFaults(fault, 30)
 67 }
```

### Scenarios 

This test defines three [scenarios](https://k6.io/docs/using-k6/scenarios) to be executed. The `base` scenario (lines 71-79) applies the test load to the target application for `30s` invoking the `default` function and it is used to set a baseline for the application's performance. The `disrupt` scenario (lines 80-86) is executed starting at the `30` seconds of the test run. It invokes once the `disrupt` function to inject a fault in the HTTP requests of the target application. The `faults` scenario (lines 87-95) is also executed starting at the `30` seconds of the test run, reproducing the same workload than the `base` scenario but now under the effect of the faults introduced by the `disrupt` scenario.

```javascript
 70     scenarios: {
 71         base: {
 72             executor: 'constant-arrival-rate',
 73             rate: 100,
 74             preAllocatedVUs: 10,
 75             maxVUs: 100,
 76             exec: "default",
 77             startTime: '0s',
 78             duration: "30s",
 79         },
 80         disrupt: {
 81             executor: 'shared-iterations',
 82             iterations: 1,
 83             vus: 1,
 84             exec: "disrupt",
 85             startTime: "30s",
 86         },
 87         faults: {
 88             executor: 'constant-arrival-rate',
 89             rate: 100,
 90             preAllocatedVUs: 10,
 91             maxVUs: 100,
 92             exec: "default",
 93             startTime: '30s',
 94             duration: "30s",
 95         }
 96      },
 ```

 > Notice that the `disrupt` scenario uses a `shared-iterations` executor with one iteration and one `VU`. This setting ensures the `disrupt` function is executed only once. Executing this function multiples times concurrently may have unpredictable results.

In order to facilitate the comparison of the results of each scenario, thresholds are defined (lines 97-102) for the `http_req_duration` and the `http_req_failed` metrics for each scenario. 

```javascript
 97      thresholds: {
 98         'http_req_duration{scenario:base}': [],
 99         'http_req_duration{scenario:faults}': [],
100         'http_req_failed{scenario:base}': [],
101         'http_req_failed{scenario:faults}': [],
102      },
```

### Results

When the code above is executed we get an output similar to the one shown below. 


```bash
$ ./build/k6 run examples/httpbin/disrupt-pod.js

          /\      |‾‾| /‾‾/   /‾‾/   
     /\  /  \     |  |/  /   /  /    
    /  \/    \    |     (   /   ‾‾\  
   /          \   |  |\  \ |  (‾)  | 
  / __________ \  |__| \__\ \_____/ .io

  execution: local
     script: examples/httpbin/disrupt-pod.js
     output: -

  scenarios: (100.00%) 3 scenarios, 201 max VUs, 11m0s max duration (incl. graceful stop):
           * base: 100.00 iterations/s for 30s (maxVUs: 10-100, exec: default, gracefulStop: 30s)
           * disrupt: 1 iterations shared among 1 VUs (maxDuration: 10m0s, exec: disrupt, startTime: 30s, gracefulStop: 30s)
           * load: 100.00 iterations/s for 30s (maxVUs: 10-100, exec: default, startTime: 30s, gracefulStop: 30s)


running (01m05.4s), 000/031 VUs, 5992 complete and 0 interrupted iterations
base    ✓ [======================================] 000/013 VUs  30s             100.00 iters/s
disrupt ✓ [======================================] 1 VUs        00m31.1s/10m0s  1/1 shared iters
load    ✓ [======================================] 000/017 VUs  30s             100.00 iters/s

     █ setup

     █ teardown

     data_received..................: 2.5 MB 38 kB/s
     data_sent......................: 533 kB 8.2 kB/s
     dropped_iterations.............: 10     0.15296/s
     http_req_blocked...............: avg=8.88µs   min=2.31µs   med=5.99µs   max=500.43µs p(90)=8.21µs   p(95)=9.45µs  
     http_req_connecting............: avg=1.8µs    min=0s       med=0s       max=368.21µs p(90)=0s       p(95)=0s      
     http_req_duration..............: avg=122.65ms min=50.3ms   med=103.73ms max=169.17ms p(90)=154.49ms p(95)=154.82ms
       { expected_response:true }...: avg=126.31ms min=101.58ms med=103.89ms max=169.17ms p(90)=154.52ms p(95)=154.85ms
       { scenario:base }...............: avg=103.12ms min=101.58ms med=103.08ms max=125.88ms p(90)=103.83ms p(95)=104.08ms
       { scenario:faults }...............: avg=142.22ms min=50.3ms   med=153.86ms max=169.17ms p(90)=154.83ms p(95)=155.17ms
     http_req_failed................: 4.85%  ✓ 291       ✗ 5700
       { scenario:base }...............: 0.00%  ✓ 0         ✗ 2998
       { scenario:faults }...............: 9.72%  ✓ 291       ✗ 2702
     http_req_receiving.............: avg=100.45µs min=26.6µs   med=88.68µs  max=427.17µs p(90)=154.15µs p(95)=171.03µs
     http_req_sending...............: avg=30.51µs  min=11.13µs  med=29.37µs  max=678.86µs p(90)=39.01µs  p(95)=43.96µs 
     http_req_tls_handshaking.......: avg=0s       min=0s       med=0s       max=0s       p(90)=0s       p(95)=0s      
     http_req_waiting...............: avg=122.52ms min=50.22ms  med=103.57ms max=169.07ms p(90)=154.37ms p(95)=154.7ms 
     http_reqs......................: 5991   91.638524/s
     iteration_duration.............: avg=128.65ms min=11.95ms  med=103.9ms  max=31.07s   p(90)=154.66ms p(95)=154.98ms
     iterations.....................: 5992   91.65382/s
     vus............................: 1      min=0       max=18
     vus_max........................: 31     min=21      max=31

```

> It may happen that you see the following error message during the test execution: <p>
> WARN[0035] Request Failed error="read tcp 172.18.0.1:43564->172.18.255.200:80: read: connection reset by peer".<p>
>This is normal and means that one request was "in transit" at the time the faults were injected, causing the request to fail due to a network connection reset.


Let's take a closer look at the results for the requests on each scenario. We can observe that he `base` scenario has a median around `100ms` and an error rate of `0%`  while the `faults` scenario has a median around `150ms` and an error rate of nearly `10%`, matching the definition of the faults defined in the disruptor.

```
       { scenario:base }...............: avg=103.12ms min=101.58ms med=103.08ms max=125.88ms p(90)=103.83ms p(95)=104.08ms
       { scenario:faults }...............: avg=142.22ms min=50.3ms   med=153.86ms max=169.17ms p(90)=154.83ms p(95)=155.17ms
     http_req_failed................: 4.85%  ✓ 291       ✗ 5700
       { scenario:base }...............: 0.00%  ✓ 0         ✗ 2998
       { scenario:faults }...............: 9.72%  ✓ 291       ✗ 2702
  ```


# Architecture

The xk6-disruptor consists of two main components: a k6 extension and the xk6-disruptor-agent. The xk6-disruptor-agent is a command line tool that can inject disruptions in the target system where it runs. The xk6-disruptor extension provides an API for injecting faults into a target system using the xk6-disruptor as a backend tool. The xk6-disruptor extension will install the agent in the target and send commands in order to inject the desired faults.

 The xk6-disruptor-agent is provided as an Docker image that can be pulled from the [xk6-disruptor repository](https://github.com/grafana/xk6-disruptor/pkgs/container/xk6-disruptor-agent) as or [build locally](#building-the-xk6-disruptor-agent-image).

## xk6-disruptor-agent

The agent offers a series of commands that inject different types of disruptions described in the next sections.

### HTTP

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

## Building the xk6-disruptor-agent image

If you modify the `xk6-disruptor-agent` you have to build the image and made it available in the test environment.

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

In order to facilitate the development of e2e tests, some helper functions have been created in the `pkg/testutils/e2e/fixtures` package for creating test resources, including a test cluster, and in `pkg/testutils/e2e/checks` package for verifying conditions during the test. We strongly encourage to keep adding reusable functions to these helper packages instead of implementing fixtures and validations for each test, unless strictly necessarily.

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
