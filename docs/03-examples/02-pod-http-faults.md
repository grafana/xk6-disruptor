# Introduce faults in http request to a pod

The example at [examples/httpbin/disrupt-pod.js](/examples/httpbin/disrupt-pod.js) shows how [PodDisruptor](../02-api/02-pod-disruptor.md) can be used for testing the effect of disruptions in the HTTP requests served by a pod. The example deploys a pod running the [httpbin](https://httpbin.org), a simple request/response application that offers endpoints for testing different HTTP request. The test consists in two load generating scenarios: one for obtaining baseline results and another for checking the effect of the faults introduced by the `PodDisruptor`, and one additional scenario for injecting the faults.

Next sections examine the sample code below in detail, describing the different steps in the test life-cycle.

## Initialization

The initialization code imports the external dependencies required by the test. The `Kubernetes` class imported from the `xk6-kubernetes` extension (line 1) provides functions for handling Kubernetes resources. The `PodDisruptor` class imported from the `xk6-disruptor` extension (line 2) provides functions for injecting faults in pods. The [k6/http](https://k6.io/docs/javascript-api/k6-http/) module (line 3) provides functions for executing HTTP requests. 

The built-in [open](https://k6.io/docs/javascript-api/init-context/open) function is used for reading the YAML manifests of the Kubernetes resources needed by test (lines 7-9). 


Finally some constants are defined: the name of the pod and service running the `httpbin` application (line 10), the namespace on which the application is running (line 11) and the timeout used for waiting the resources to be ready (line 12).

```javascript
  1 import { Kubernetes } from 'k6/x/kubernetes';
  2 import { PodDisruptor } from 'k6/x/disruptor';
  3 import  http from 'k6/http';
  4 import exec from 'k6/execution';
  5
  6 // read manifests for resources used in the test
  7 const podManifest = open("./manifests/pod.yaml")
  8 const svcManifest = open("./manifests/service.yaml")
  9 const nsManifest  = open("./manifests/namespace.yaml")
 10 const app = "httpbin"
 11 const namespace = "httpbin-ns"
 12 const timeout = 30
 ```

## Setup and teardown

The `setup` function creates the Kubernetes resources needed by the test using the `apply` function provided by the `Kubernetes` class. The resources are defined as `yaml` manifests imported in the init code. It creates a namespace (line 18) for isolating the test from other tests running in the same cluster, then deploys the application as a pod (line 21) and waits until the pod is ready using the helper function `waitPodRunning` (line 22). The pod is exposed as a service (line 29) and the `getExternalIP` function is used for waiting until the service is assigned an IP for being accessed from outside the cluster (line 30). This IP address is then returned as part of the setup data to be used by the test code (line 37-39).

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
 24          k8s.delete("Namespace", namespace)
 25          exec.test.abort("pod "+ app + " not ready after " + timeout + " seconds")
 26    }
 27
 28    // expose deployment as a service
 29    k8s.apply(svcManifest)
 30    const ip = k8s.helpers(namespace).getExternalIP(app, timeout)
 31    if (ip == "") {
 32        k8s.delete("Namespace", namespace)
 33        exec.test.abort("service " + app + " have no external IP after " + timeout + " seconds")
 34    }
 35
 36    // pass service ip to scenarios
 37    return {
 38        srv_ip: ip,
 39    }
 40 }
 ```

> The time required for creating the httpbin pod and exposing it as a service varies significantly between environments. Times of 1 minute or more are not uncommon.

> ⚠️ If you get the message `test aborted: service httpbin have no external IP after 30 seconds` verify your cluster is properly configured for exposing `LoadBalancer` services. Check the [exposing your application](/docs/01-get-started/04-exposing-apps.md) section in the get started guide for more details.

The `teardown` function is invoked when the test ends to cleanup all resources. As all the resources created by the tests are defined in a namespace, the teardown logic only has to delete this namespace and all associated resources will be deleted (line 44).

```javascript
 42 export function teardown(data) {
 43    const k8s = new Kubernetes()
 44    k8s.delete("Namespace", namespace)
 45 }
 ```

> ⚠️ Deleting the namespace may take several seconds. If you retry the test shortly after a previous execution you may find the error `object is being deleted: namespaces "httpbin-ns" already exists`. Allow some time for the deletion to complete and retry the execution.

## Test Load

The test load is generated by the `default` function, which executes a request to the `httpbin` service using the IP address obtained int the `setup` function. The test makes requests to the endpoint `delay/0.1` which will return after `0.1` seconds (`100ms`).

```javascript
 47 export default function(data) {
 48     http.get(`http://${data.srv_ip}/delay/0.1`);
 49 }
 ```

 > The test uses the `delay` endpoint which return after the requested delay. It requests a `0.1s` (`100ms`) delay to ensure the baseline scenario (see scenarios below) has meaningful statistics for the request duration. If we were simply calling a locally deployed http server (for example `nginx`), the response time would exhibit a large variation between a few microseconds to a few milliseconds. Having `100ms` as baseline response time has proved to offer more consistent results.

## Fault injection

The `disrupt` function creates a PodDisruptor](pod-disruptor) using a selector that matches pods in the namespace `httpbin-ns` with the label `app: httpbin` (lines 52-60). 

The http faults are then injected by calling the `PodDisruptor`'s `injectHTTPFaults` method using a fault definition that introduces a delay of `50ms` on each request and an error code `500` in a `10%` of the requests (lines 63-68).

```javascript
 51 export function disrupt(data) {
 52     const selector = {
 53         namespace: namespace,
 54             select: {
 55                 labels: {
 56                     app: app
 57                 }
 58         }
 59     }
 60     const podDisruptor = new PodDisruptor(selector)
 61
 62     // delay traffic from one random replica of the deployment
 63     const fault = {
 64         average_delay: 50,
 65         error_code: 500,
 66         error_rate: 0.1
 67     }
 68     podDisruptor.injectHTTPFaults(fault, 30)
 69 }
```

## Scenarios 

This test defines three [scenarios](https://k6.io/docs/using-k6/scenarios) to be executed. The `base` scenario (lines 74-82) applies the test load to the target application for `30s` invoking the `default` function and it is used to set a baseline for the application's performance. The `disrupt` scenario (lines 83-89) is executed starting at the `30` seconds of the test run. It invokes once the `disrupt` function to inject a fault in the HTTP requests of the target application. The `faults` scenario (lines 90-98) is also executed starting at the `30` seconds of the test run, reproducing the same workload than the `base` scenario but now under the effect of the faults introduced by the `disrupt` scenario.

```javascript
 73     scenarios: {
 74         base: {
 75             executor: 'constant-arrival-rate',
 76             rate: 100,
 77             preAllocatedVUs: 10,
 78             maxVUs: 100,
 79             exec: "default",
 80             startTime: '0s',
 81             duration: "30s",
 82         },
 83         disrupt: {
 84             executor: 'shared-iterations',
 85             iterations: 1,
 86             vus: 1,
 87             exec: "disrupt",
 88             startTime: "30s",
 89         },
 90         faults: {
 91             executor: 'constant-arrival-rate',
 92             rate: 100,
 93             preAllocatedVUs: 10,
 94             maxVUs: 100,
 95             exec: "default",
 96             startTime: '30s',
 97             duration: "30s",
 98         }
 99      },
 ```

 > Notice that the `disrupt` scenario uses a `shared-iterations` executor with one iteration and one `VU`. This setting ensures the `disrupt` function is executed only once. Executing this function multiples times concurrently may have unpredictable results.

In order to facilitate the comparison of the results of each scenario, thresholds are defined (lines 100-105) for the `http_req_duration` and the `http_req_failed` metrics for each scenario. 

```javascript
100     thresholds: {
101         'http_req_duration{scenario:base}': [],
102         'http_req_duration{scenario:faults}': [],
103         'http_req_failed{scenario:base}': [],
104         'http_req_failed{scenario:faults}': [],
105      },
```

## Results

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
