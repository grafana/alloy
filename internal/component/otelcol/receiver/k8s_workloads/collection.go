package k8s_workloads

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Bound each decoded response rather than loading all full Pod specs in a
// namespace at once. Continue tokens preserve the API's consistent list view;
// any error (including an expired token) aborts the snapshot instead of emitting
// incomplete authoritative membership.
const collectionPageSize = 100

func (c *controller) listDeployments(ctx context.Context, namespace string) (*appsv1.DeploymentList, error) {
	out := &appsv1.DeploymentList{}
	opts := metav1.ListOptions{Limit: collectionPageSize}
	for {
		page, err := c.opts.client.AppsV1().Deployments(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		if opts.Continue == "" {
			out.ResourceVersion = page.ResourceVersion
		}
		for i := range page.Items {
			d := &page.Items[i]
			d.ManagedFields = nil
			d.Annotations = nil
			out.Items = append(out.Items, *d)
		}
		opts.Continue = page.Continue
		if opts.Continue == "" {
			return out, nil
		}
	}
}

func (c *controller) listReplicaSets(ctx context.Context, namespace string, needed bool) (*appsv1.ReplicaSetList, error) {
	out := &appsv1.ReplicaSetList{}
	if !needed {
		return out, nil
	}
	opts := metav1.ListOptions{Limit: collectionPageSize}
	for {
		page, err := c.opts.client.AppsV1().ReplicaSets(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for i := range page.Items {
			rs := &page.Items[i]
			if owner := metav1.GetControllerOf(rs); owner != nil {
				out.Items = append(out.Items, appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{UID: rs.UID, OwnerReferences: []metav1.OwnerReference{*owner}}})
			}
		}
		opts.Continue = page.Continue
		if opts.Continue == "" {
			return out, nil
		}
	}
}

func (c *controller) listPods(ctx context.Context, namespace string, needed bool) (*corev1.PodList, error) {
	out := &corev1.PodList{}
	if !needed {
		return out, nil
	}
	opts := metav1.ListOptions{Limit: collectionPageSize}
	for {
		page, err := c.opts.client.CoreV1().Pods(namespace).List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for i := range page.Items {
			pod := &page.Items[i]
			if owner := metav1.GetControllerOf(pod); owner != nil {
				out.Items = append(out.Items, corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{*owner}},
					Status:     corev1.PodStatus{ContainerStatuses: compactStatuses(pod.Status.ContainerStatuses), InitContainerStatuses: compactStatuses(pod.Status.InitContainerStatuses)},
				})
			}
		}
		opts.Continue = page.Continue
		if opts.Continue == "" {
			return out, nil
		}
	}
}

func compactStatuses(statuses []corev1.ContainerStatus) []corev1.ContainerStatus {
	out := make([]corev1.ContainerStatus, 0, len(statuses))
	for i := range statuses {
		s := &statuses[i]
		if s.ImageID != "" {
			out = append(out, corev1.ContainerStatus{Name: s.Name, ImageID: s.ImageID})
		}
	}
	return out
}
