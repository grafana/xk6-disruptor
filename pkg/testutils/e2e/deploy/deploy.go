// Package deploy offers helpers for deploying applications in a cluster
package deploy

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// RunPod creates a pod and waits it for be running
func RunPod(k8s kubernetes.Kubernetes, ns string, pod corev1.Pod, timeout time.Duration) error {
	_, err := k8s.Client().CoreV1().Pods(ns).Create(
		context.TODO(),
		&pod,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("error creating pod %s: %w", pod.Name, err)
	}

	running, err := k8s.PodHelper(ns).WaitPodRunning(
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

// ExposeApp deploys a pod in a namespace and exposes it as a service in a cluster
// The ingress routes request that specify the service's name as host to this service.
func ExposeApp(
	k8s kubernetes.Kubernetes,
	namespace string,
	pod corev1.Pod,
	svc corev1.Service,
	port intstr.IntOrString,
	timeout time.Duration,
) error {
	// TODO: use a context with a Deadline to control timeout
	start := time.Now()
	err := RunPod(k8s, namespace, pod, timeout)
	if err != nil {
		return fmt.Errorf("failed to create pod %s in namespace %s: %w", pod.Name, namespace, err)
	}

	timeLeft := timeout - time.Since(start)
	_, err = k8s.Client().CoreV1().Services(namespace).Create(
		context.TODO(),
		&svc,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create service %s: %w", svc.Name, err)
	}

	// wait for the service to be ready for accepting requests
	err = k8s.ServiceHelper(namespace).WaitServiceReady(context.TODO(), svc.Name, timeLeft)
	if err != nil {
		return fmt.Errorf("error waiting for service %s: %w", svc.Name, err)
	}

	ingress := builders.NewIngressBuilder(svc.Name, port).
		WithNamespace(namespace).
		WithHost(svc.Name).
		Build()

	_, err = k8s.Client().NetworkingV1().Ingresses(namespace).Create(
		context.TODO(),
		&ingress,
		metav1.CreateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to create ingress %s: %w", ingress.Name, err)
	}

	// wait for the ingress to be ready for accepting requests
	timeLeft = timeout - time.Since(start)
	err = k8s.ServiceHelper(namespace).WaitIngressReady(context.TODO(), svc.Name, timeLeft)
	if err != nil {
		return fmt.Errorf("error waiting for ingress %s: %w", ingress.Name, err)
	}

	return nil
}
