import { Kubernetes } from 'k6/x/kubernetes';
import { PodDisruptor } from 'k6/x/disruptor';
import  http from 'k6/http';

// read manifests for resources used in the test
const podManifest = open("./manifests/pod.yaml")
const svcManifest = open("./manifests/service.yaml")
const nsManifest  = open("./manifests/namespace.yaml")

const app = "httpbin"
const namespace = "httpbin-ns"
const timeout = 10

export function setup() {
    const k8s = new Kubernetes()

    // create namespace for isolating test
    k8s.apply(nsManifest)

    // create a test deployment and wait until is ready
    k8s.apply(podManifest)
    const ready = k8s.helpers(namespace).waitPodRunning(app, timeout)
    if (!ready) {
        throw "aborting test. Pod "+ app + " not ready after " + timeout + " seconds"
    }

    // expose deployment as a service
    k8s.apply(svcManifest)
    const ip = k8s.helpers(namespace).getExternalIP(app, timeout)
    if (ip == "") {
        throw "aborting test. Service " + app + " have no external IP after " + timeout + " seconds"
    }

    // pass service ip to scenarios
    return {
        srv_ip: ip,
    }
}

export function teardown(data) {
    const k8s = new Kubernetes()
    k8s.delete("Namespace", namespace)
}

export default function(data) {
    http.get(`http://${data.srv_ip}/delay/0.1`);
 }

export function disrupt(data) {
  const selector = {
      namespace: namespace,
      select: {
          labels: {
              app: app
          }
      }
  }
  const podDisruptor = new PodDisruptor(selector)

  // delay traffic from one random replica of the deployment
  const fault = {
      average_delay: 50,
      error_code: 500,
      error_rate: 0.1
  }
  podDisruptor.injectHttpFaults(fault, 30)
}

export const options = {
    scenarios: {
        base: {
          executor: 'constant-arrival-rate',
          rate: 100,
          preAllocatedVUs: 10,
          maxVUs: 100,
          exec: "default",
          startTime: '0s',
          duration: "30s",
        },
        disrupt: {
            executor: 'shared-iterations',
            iterations: 1,
            vus: 1,
            exec: "disrupt",
            startTime: "30s",
       },
       faults: {
            executor: 'constant-arrival-rate',
            rate: 100,
            preAllocatedVUs: 10,
            maxVUs: 100,
            exec: "default",
            startTime: '30s',
            duration: "30s",
        }
    },
    thresholds: {
      'http_req_duration{scenario:base}': [],
      'http_req_duration{scenario:faults}': [],
      'http_req_failed{scenario:base}': [],
      'http_req_failed{scenario:faults}': [],
    },
}
