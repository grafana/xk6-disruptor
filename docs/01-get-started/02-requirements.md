# Requirements

The `xk6-disruptor` is a `k6` extension. In order to use it in a `k6` test script, it is necessary to use a custom build of `k6` that includes the disruptor. See the [Installation](./03-installation.md) section  for instructions on how to get this custom build.

The `xk6-disruptor` needs to interact with the Kubernetes cluster on which the application under test is running. In order to do so, you must have the credentials to access the cluster in a [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) file. Ensure this file is pointed by the `KUBECONFIG` environment variable or it is located at the default location `$HOME/.kube/config`.

`xk6-disruptor` requires the [grafana/xk6-disruptor-agent](https://github.com/grafana/xk6-disruptor/pkgs/container/xk6-disruptor-agent) image for injecting the [disruptor agent](../04-development/02-architecture.md#xk6-disruptor-agent) into the disruption targets. Kubernetes clusters can be configured to restrict the download of images from public repositories. You need to ensure this image is available in the cluster where the application under test is running. Additionally, the xk6-disruptor-agent must run with network access privileges. Kubernetes clusters [can be configured to restrict the privileges of containers](https://kubernetes.io/docs/concepts/security/pod-security-admission/). If you find an error similar to the one shown below when using the xk6-disruptor, contact your cluster administrator and request the necessary privileges.

> ERROR[0000] error creating PodDisruptor: pods "nginx" is forbidden: violates PodSecurity "baseline:latest": non-default capabilities (container "xk6-agent" must not include "NET_ADMIN", "NET_RAW" in securityContext.capabilities.add)


You also need to ensure your test application is accessible from the machine where the tests run. See the [Exposing your application](./04-exposing-apps.md) section for instructions on how to make your application available from outside the Kubernetes cluster.
