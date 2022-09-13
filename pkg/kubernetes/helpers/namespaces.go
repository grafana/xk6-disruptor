package helpers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceHelper interface {
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
