package harness

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ensureCleanNamespace(ctx *TestContext) error {
	return step("prepare namespace "+ctx.Namespace, func() error {
		_, getErr := ctx.client.CoreV1().Namespaces().Get(context.Background(), ctx.Namespace, metav1.GetOptions{})
		if getErr == nil {
			if err := runCommand("kubectl", "delete", "namespace", ctx.Namespace, "--wait=true", "--timeout=10m"); err != nil {
				return fmt.Errorf("delete namespace %q: %w", ctx.Namespace, err)
			}
		} else if !apierrors.IsNotFound(getErr) {
			return fmt.Errorf("get namespace %q: %w", ctx.Namespace, getErr)
		}

		_, err := ctx.client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ctx.Namespace,
				Labels: map[string]string{
					"alloy": "yes",
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create namespace %q: %w", ctx.Namespace, err)
		}
		return nil
	})
}

func deleteNamespace(namespace string) error {
	return step("cleanup namespace "+namespace, func() error {
		return runCommand("kubectl", "delete", "namespace", namespace, "--ignore-not-found=true", "--wait=true", "--timeout=10m")
	})
}
