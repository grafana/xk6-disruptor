package helpers

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NamespaceHelper defines helper methods for handling namespaces
type NamespaceHelper interface {
	// CreateRandomNamespace creates a namespace with a random name starting with
	// the provided prefix and returns its name
	CreateRandomNamespace(ctx context.Context, prefix string) (string, error)
}

// helpers struct holds the data required by the helpers
type nsHelper struct {
	client kubernetes.Interface
}

// NewNamespaceHelper creates a namespace helper
func NewNamespaceHelper(client kubernetes.Interface) NamespaceHelper {
	return &nsHelper{
		client: client,
	}
}

func (h *nsHelper) CreateRandomNamespace(ctx context.Context, prefix string) (string, error) {
	ns, err := h.client.CoreV1().Namespaces().Create(
		ctx,
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
