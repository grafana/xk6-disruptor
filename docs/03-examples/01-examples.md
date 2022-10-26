# Examples

In the next sections we present some examples of how to use the `xk6-disruptor` extension to introduce faults in `k6` tests. Some examples make use the [k6-kubernetes](http://github.com/grafana/xk6-kubernetes) extension. Ensure the custom `k6` binary you are using is built with both `xk6-disruptor` and `xk6-kubernetes` extensions. See the [Installation guide](../01-get-started/03-installation.md) for details.

Also, check your test environment satisfies the requirements described in the [get started guide](../01-get-started/02-requirements.md). In particular, check you have the credentials for accessing the Kubernetes cluster used for the test properly configured and that this cluster is configured for exposing your application using an external IP.


[Injecting HTTP faults to a Pod](./02-pod-http-faults.md)