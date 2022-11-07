# Service Disruptor

The `ServiceDisruptor` class allows the injection of different types of faults in the pods that back a Kubernetes service. The disruptor uses the selector attribute in the service definition for selecting the target pods.
 
`constructor`: creates a service disruptor

Parameters:
- service: name of the service
- namespace: namespace on which the service is defined
- options: options for controlling the behavior of the disruptor

The `options` control the creation and behavior of the service disruptor:
- injectTimeout: maximum time for waiting the [xk6-disruptor-agent](../04-development/02-architecture.md#xk6-disruptor-agent) to be ready in the target pods, in seconds (default 30s). Zero value forces default. Negative values force no waiting.


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
- errorBody: body to be returned when an error in injected
- exclude: list of urls to be excluded from disruption (e.g. /health)

The injection of the fault is controlled by the following options:
  - proxyPort: port the agent will use to listen for requests in the target pods ( default `8080`)
  - iface: network interface where the agent will capture the traffic ( default `eth0`)

`targets`: returns the list of target pods for the disruptor.

Example: [service_disruptor.js](/examples/service_disruptor.js) shows how to create a disruptor for the `nginx` service and inject a delay of 100ms and a 10% of requests returning a http response code 500. 

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
