# Pod Disruptor

The `PodDisruptor` class allows the injection of different types of faults in pods. The target pod(s) are defined by means of a pod selector.

The faults are injected with the help of a [k6-disruptor-agent](../development/architecture.md#xk6-disruptor-agent) attached on each of the target pods. The agent is capable of intercepting traffic directed to the pod and apply the desired effect.
 
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
- injectTimeout: maximum time for waiting the [xk6-disruptor-agent](../04-development/02-architecture.md#xk6-disruptor-agent) to be ready in the target pods, in seconds (default 30s). Zero value forces default. Negative values force no waiting.


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
- errorBody: body to be returned when an error is injected
- exclude: comma-separated list of urls to be excluded from disruption (e.g. /health)

The injection of the fault is controlled by the following options:
  - proxyPort: port the agent will use to listen for requests in the target pods ( default `8080`)
  - iface: network interface where the agent will capture the traffic ( default `eth0`)

`targets`: returns the list of target pods for the disruptor.

Example: [pod_disruptor.js](/examples/pod_disruptor.js) shows how to create a selector that matches all pods in the `default` namespace with the `run=nginx` label and inject a delay of 100ms and a 10% of requests returning a http response code 500.


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
