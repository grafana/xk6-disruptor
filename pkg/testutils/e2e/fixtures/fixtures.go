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

// BuildGrpcpbinPod returns the definition for deploying grpcbin as a Pod
func BuildGrpcpbinPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "grpcbin",
			Labels: map[string]string{
				"app": "grpcbin",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "grpcbin",
					Image:           "moul/grpcbin",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 9000,
						},
					},
				},
			},
		},
	}
}

// BuildHttpbinService returns a Service definition that exposes httpbin pods
func BuildHttpbinService() *corev1.Service {
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
					Name: "http",
					Port: 80,
				},
			},
		},
	}
}

// BuildGrpcbinService returns a Service definition that exposes grpcbin pods at the node port 30000
func BuildGrpcbinService(nodePort uint) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "grpcbin",
			Labels: map[string]string{
				"app": "grpcbin",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "grpcbin",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "grpcbin",
					Port:     9000,
					NodePort: int32(nodePort),
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
func BuildNginxService() *corev1.Service {
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
					Name: "http",
					Port: 80,
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
		return fmt.Errorf("failed to create service %s: %w", svc.Name, err)
	}

	// wait for the service to be ready for accepting requests
	err = k8s.NamespacedHelpers(ns).WaitServiceReady(context.TODO(), svc.Name, timeout)
	if err != nil {
		return fmt.Errorf("error waiting for service %s: %w", svc.Name, err)
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
		return fmt.Errorf("error creating pod %s: %w", pod.Name, err)
	}

	running, err := k8s.NamespacedHelpers(ns).WaitPodRunning(
		context.TODO(),
		pod.Name,
		timeout,
	)
	if err != nil {
		return fmt.Errorf("error waiting for pod %s: %w", pod.Name, err)
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
		return fmt.Errorf("failed to create pod %s in namespace %s: %w", pod.Name, ns, err)
	}

	timeLeft := timeout - time.Since(start)
	return ExposeService(k8s, ns, svc, timeLeft)
}

// BuildCluster builds a cluster with the xk6-disruptor-agent image preloaded and
// the given node ports exposed
func BuildCluster(name string, ports ...cluster.NodePort) (*cluster.Cluster, error) {
	config, err := cluster.NewConfig(
		name,
		cluster.Options{
			Images:    []string{"ghcr.io/grafana/xk6-disruptor-agent:latest"},
			Wait:      time.Second * 60,
			NodePorts: ports,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster config: %w", err)
	}

	return config.Create()
}
