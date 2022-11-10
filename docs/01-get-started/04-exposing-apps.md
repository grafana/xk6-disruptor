# Exposing your application

In order to access your application from the test scripts, it must be assigned an external IP in the cluster it is running at. This can be accomplished in different ways depending on the platform you used for deploying the application. Next sections explain different approaches.

Once your application has an external IP you can retrieve it in your script's `setup` function using the [getExternalIP](https://github.com/grafana/xk6-kubernetes#helpers) helper function offered by the [xk6-kubernetes](https://github.com/grafana/xk6-kubernetes) extension and pass it to the test in the setup data:

```javascript
import { Kubernetes } from 'k6/x/kubernetes';

const service = "service-name"
const namespace  = "service-namespace"

export function setup() {
        const k8s = new Kubernetes()

        const ip = k8s.helpers(namespace).getExternalIP(service)

        return {
            srvIP: ip
        }
}

export default function(data) {
     http.get(`http://${data.srvIP}/path/to/endpoint`);
}
```

## As a LoadBalancer service
A service of type [`LoadBalancer`](https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/) receives an external IP from an external load balancer provider. The load balancer is configured in different ways depending on the platform your cluster is deployed at and the configuration of the cluster. In the following sections we provide guidelines for exposing your application when running in common development environments. If your cluster is deployed in a public cloud, check your cloud provider's documentation.

If the service you want to access in your tests is not defined as a load balancer, you can change the service type with the command shown below. The service will then receive an external IP.

```bash
$ kubectl patch svc <service name> -p '{"spec": {"type": "LoadBalancer"}}'
```


### Configuring a LoadBalancer in Kind
[Kind](https://kind.sigs.k8s.io/) is a tool for running local Kubernetes clusters using Docker container to emulate “nodes”. It may be used for local development or CI. Services deployed in a kind cluster can be exposed to be accessed from the host machine [using metallb as a load balancer](https://kind.sigs.k8s.io/docs/user/loadbalancer).

### Configuring a LoadBalancer in Minikube

[Minikube](https://github.com/kubernetes/minikube) implements a local Kubernetes cluster supporting different technologies for virtualizing the cluster's infrastructure, such as containers, VMs or running in bare metal.

Minikube's tunnel command runs as a process, creating a network route on the host to the service CIDR of the cluster using the cluster’s IP address as a gateway. The tunnel command exposes the external IP directly to any program running on the host operating system.

```console
$ minikube tunnel
```