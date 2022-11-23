# xk6-disruptor

</br>
</br>

<p align="center">
  <img src="assets/logo.png" alt="xk6-disruptor" width="300"></a>
  <br>
  "Like Unit Testing, but for <strong>Reliability</strong>"
  <br>
</p>
<p align="center">  
</div>

<blockquote align="center">
⚠️ <strong>Important</strong> ⚠️ 
<br>This project is still in its early stages and under active development.
<br>The API is subject to change, and there may be bugs.
<br>Use at your own risk. Thanks!
</blockquote>

This extension adds fault injection capabilities to [Grafana k6](https://github.com/grafana/k6). It implements the ideas of the Chaos Engineering discipline and enables Grafana k6 users to test their system's reliability under turbulent conditions.

## Why another Chaos Engineering tool?

Compared to other tools in the space, this one is purposely designed and built to provide the best experience for developers trying to make their systems more reliable:

- Everything as code.
  - No need to mess with YAML or learn a new DSL.
- Fast to adopt with no day-two surprises.
  - No need to deploy and maintain a fleet of agents or operators.
- Easy to extend and integrate with other types of tests.
  - No need to try to glue five tools together to get the job done.

All that plus great docs and sane APIs. Also, this project has been built to be a good citizen in the Grafana k6 ecosystem by:

- Working well with other extensions.
- Working well with k6's core concepts and features.

You can check this out in the following example:

```js
import { PodDisruptor } from "k6/x/disruptor";

export default function () {
    // Create a new pod disruptor with a selector 
    // that matches pods from the "default" namespace with the label "app=my-app"
    const disruptor = new PodDisruptor({
        namespace: "default",
        select: { labels: { app: "my-app" } },
    });

    // Check that there is at least one target
    const targets = disruptor.targets();
    if (targets.length != 1) {
        throw new Error("expected list to have one target");
    }

    // Disrupt the targets by injecting HTTP faults into them for 30 seconds
    disruptor.injectHTTPFaults(
        {
            averageDelay: 500,
            errorRate: 0.1,
            errorCode: 500,
        },
        30
    );
}
```

## Features

The project, at this time, is intended to test systems running in Kubernetes. Other platforms are not supported at this time.

Right now, it offers an [API](./docs/02-api/01-api.md) for creating disruptors that target one specific type of the component (e.g., Pods) and is capable of injecting different kinds of faults, such as errors in HTTP requests served by that component. Currently, disruptors exist for [Pods](/docs/02-api/02-pod-disruptor.md) and [Services](/docs/02-api/03-service-disruptor.md), but others will be introduced in the future as well as additional types of faults for the existing disruptors.

## Use cases

The main use case for xk6-disruptor is to test the resiliency of an application of diverse types of disruptions by reproducing their effects without reproducing their root causes. For example, inject delays in the HTTP requests an application makes to a service without having to stress or interfere with the infrastructure (network, nodes) on which the service runs or affect other workloads in unexpected ways.

In this way, xk6-disruptor make reliability tests repeatable and predictable while limiting their blast radius. These are essential characteristics to incorporate these tests in the test suits of applications deployed on shared infrastructures such as staging environments.

## Learn more

Check the [get started guide](/docs/01-get-started/01-get-started.md) for instructions on how to install and use `xk6-disruptor`.

If you are interested in contributing to the development of this project, check the [contributing guide](/docs/04-development/01-contributing.md)
