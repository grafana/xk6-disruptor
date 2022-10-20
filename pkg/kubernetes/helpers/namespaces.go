package helpers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespaceHelper defines helper methods for handling namespaces
type NamespaceHelper interface {
	// CreateRandomNamespace creates a namespace with a random name starting with
        // the provided prefix and returns its name
	CreateRandomNamespace(prefix string) (string, error)
}

func (h *helpers) CreateRandomNamespace(prefix string) (string, error) {
	ns, err := h.client.CoreV1().Namespaces().Create(
		h.ctx,
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: prefix},
		},
		metav1.CreateOptions{},
	)
	if err != nil {
		return "", err
	}

	return ns.GetName(), nil
}
