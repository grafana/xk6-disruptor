package disruptors

import (
	corev1 "k8s.io/api/core/v1"
)

const (
	testNamespace = "test-ns"
)

// simplified description of a Service used for building a corev1.Service Object
type serviceDesc struct {
	name      string
	namespace string
	ports     []corev1.ServicePort
	selector  map[string]string
}

// simplified definition of EndPoint used to build a corev1.Endpoint object
// lists the names of pods that expose the given EndpointPort
type endpoint struct {
	ports []corev1.EndpointPort
	pods  []string
}

// podDesc describes a pod for a test
type podDesc struct {
	name      string
	namespace string
	labels    map[string]string
}
