# xk6-disruptor

The xk6-disruptor is a [k6](https://k6.io) extension providing fault injection capabilities to test system's reliability under turbulent conditions. Think of it as "like unit testing, but for reliability". 

This project aims to aid developers in building reliable systems, implementing the goals of "Chaos Engineering" discipline in a k6 way - with the best developer experience as its primary objective. 

xk6-disruptor is intended for systems running in kubernetes. Other platforms are not supported at this time.

The extension offers an [API](./docs/02-api/01-api.md) for creating disruptors that target one specific type of component (for example, Pods) and are capable of injecting different types of faults such as errors in HTTP requests served by that component. Currently disruptors exist for [Pods](./docs/02-api/02-pod-disruptor.md] and [Services](./docs/02-api/03-service-disruptor.md), but others will be introduced in the future as well as additional types of faults for the existing disruptors.


> ⚠️  xk6-disruptor is in the alpha stage, undergoing active development. We do not guarantee API compatibility between releases - your k6 scripts may need to be updated on each release until this extension reaches v1.0 release.


## Learn more

Check the [get started guide](./docs/01-get-started/01-get-started.md) for instructions on how to install and use `xk6-disruptor`.

If you are interested in contributing with the development of this project, check the [contributing guide](./docs/04-development/01-contributing.md)

