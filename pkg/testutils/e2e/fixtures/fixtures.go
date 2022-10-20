// Package fixtures implements fixtures for e2e tests
package fixtures

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildHttpbinPod returns the definition for deploying Httpbin as a Pod
func BuildHttpbinPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "httpbin",
			Labels: map[string]string{
				"app": "httpbin",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "httpbin",
					Image:           "kennethreitz/httpbin",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}
}

// BuildHttpbinService returns a Service definition that exposes httpbin pods at the node port 32080
func BuildHttpbinService(nodeport int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "httpbin",
			Labels: map[string]string{
				"app": "httpbin",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "httpbin",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					NodePort: nodeport,
				},
			},
		},
	}
}

// BuildBusyBoxPod returns the definition of a Pod that runs busybox and waits 5min before completing
func BuildBusyBoxPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "busybox",
			Labels: map[string]string{
				"app": "busybox",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"sleep", "300"},
				},
			},
		},
	}
}

// BuildPausedPod returns the definition of a Pod that runs the paused image in a container
// creating a "no-op" dummy Pod.
func BuildPausedPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "paused",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "paused",
					Image:           "k8s.gcr.io/pause",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}
}

// BuildNginxPod returns the definition of a Pod that runs Nginx
func BuildNginxPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"app": "nginx",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "nginx",
					Image:           "nginx",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}
}

// BuildNginxService returns the definition of a Service that exposes the nginx pod(s)
func BuildNginxService(nodeport int32) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"app": "nginx",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "nginx",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					NodePort: nodeport,
				},
			},
		},
	}
}

// ExposeService exposes a service in the given namespace and waits for it to be ready
func ExposeService(k8s kubernetes.Kubernetes, ns string, svc *corev1.Service, timeout time.Duration) error {
	_, err := k8s.CoreV1().Services(ns).Create(
		context.TODO(),
		svc,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create service %s: %v", svc.Name, err)
	}

	// wait for the service to be ready for accepting requests
	err = k8s.NamespacedHelpers(ns).WaitServiceReady(svc.Name, timeout)
	if err != nil {
		return fmt.Errorf("error waiting for service %s: %v", svc.Name, err)
	}

	return nil
}

// RunPod creates a pod and waits it for be running
func RunPod(k8s kubernetes.Kubernetes, ns string, pod *corev1.Pod, timeout time.Duration) error {
	_, err := k8s.CoreV1().Pods(ns).Create(
		context.TODO(),
		pod,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("error creating pod %s: %v", pod.Name, err)
	}

	running, err := k8s.NamespacedHelpers(ns).WaitPodRunning(
		pod.Name,
		timeout,
	)
	if err != nil {
		return fmt.Errorf("error waiting for pod %s: %v", pod.Name, err)
	}
	if !running {
		return fmt.Errorf("pod %s not ready after %f: ", pod.Name, timeout.Seconds())
	}

	return nil
}

// DeployApp deploys a pod in a namespace and exposes it as a service in a cluster
func DeployApp(
	k8s kubernetes.Kubernetes,
	ns string,
	pod *corev1.Pod,
	svc *corev1.Service,
	timeout time.Duration,
) error {
	start := time.Now()
	err := RunPod(k8s, ns, pod, timeout)
	if err != nil {
		return fmt.Errorf("failed to create pod %s in namespace %s: %v", pod.Name, ns, err)
	}

	timeLeft := time.Duration(timeout - time.Since(start))
	return ExposeService(k8s, ns, svc, timeLeft)
}

// BuildCluster builds a cluster exposing port 32080 and with the required images preloaded
func BuildCluster(name string) (*cluster.Cluster, error) {
	// map node ports in the range 32080-32089 to host ports
	nodePorts := []cluster.NodePort{}
	for port := 32080; port < 32090; port++ {
		nodePorts = append(nodePorts, cluster.NodePort{HostPort: int32(port), NodePort: int32(port)})
	}

	config, err := cluster.NewConfig(
		name,
		cluster.Options{
			NodePorts: nodePorts,
			Images:    []string{"ghcr.io/grafana/xk6-disruptor-agent:latest"},
			Wait:      time.Second * 60,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster config: %w", err)
	}

	return config.Create()
}
