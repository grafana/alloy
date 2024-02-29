package clientutil

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	apps_v1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var invalidDNS1123Characters = regexp.MustCompile("[^-a-z0-9]+")

// SanitizeVolumeName ensures that the given volume name is a valid DNS-1123 label
// accepted by Kubernetes.
//
// Copied from github.com/prometheus-operator/prometheus-operator/pkg/k8sutil.
func SanitizeVolumeName(name string) string {
	name = strings.ToLower(name)
	name = invalidDNS1123Characters.ReplaceAllString(name, "-")
	if len(name) > validation.DNS1123LabelMaxLength {
		name = name[0:validation.DNS1123LabelMaxLength]
	}
	return strings.Trim(name, "-")
}

// CreateOrUpdateSecret applies the given secret against the client.
func CreateOrUpdateSecret(ctx context.Context, c client.Client, s *v1.Secret) error {
	var exist v1.Secret
	err := c.Get(ctx, client.ObjectKeyFromObject(s), &exist)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve existing service: %w", err)
	}

	if k8s_errors.IsNotFound(err) {
		err := c.Create(ctx, s)
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	} else {
		s.ResourceVersion = exist.ResourceVersion
		s.SetOwnerReferences(mergeOwnerReferences(s.GetOwnerReferences(), exist.GetOwnerReferences()))
		s.SetLabels(mergeMaps(s.Labels, exist.Labels))
		s.SetAnnotations(mergeMaps(s.Annotations, exist.Annotations))

		err := c.Update(ctx, s)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return fmt.Errorf("failed to update service: %w", err)
		}
	}

	return nil
}

// CreateOrUpdateService applies the given svc against the client.
func CreateOrUpdateService(ctx context.Context, c client.Client, svc *v1.Service) error {
	var exist v1.Service
	err := c.Get(ctx, client.ObjectKeyFromObject(svc), &exist)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve existing service: %w", err)
	}

	if k8s_errors.IsNotFound(err) {
		err := c.Create(ctx, svc)
		if err != nil {
			return fmt.Errorf("failed to create service: %w", err)
		}
	} else {
		svc.ResourceVersion = exist.ResourceVersion
		svc.Spec.IPFamilies = exist.Spec.IPFamilies
		svc.SetOwnerReferences(mergeOwnerReferences(svc.GetOwnerReferences(), exist.GetOwnerReferences()))
		svc.SetLabels(mergeMaps(svc.Labels, exist.Labels))
		svc.SetAnnotations(mergeMaps(svc.Annotations, exist.Annotations))

		err := c.Update(ctx, svc)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return fmt.Errorf("failed to update service: %w", err)
		}
	}

	return nil
}

// CreateOrUpdateEndpoints applies the given eps against the client.
func CreateOrUpdateEndpoints(ctx context.Context, c client.Client, eps *v1.Endpoints) error {
	var exist v1.Endpoints
	err := c.Get(ctx, client.ObjectKeyFromObject(eps), &exist)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve existing endpoints: %w", err)
	}

	if k8s_errors.IsNotFound(err) {
		err := c.Create(ctx, eps)
		if err != nil {
			return fmt.Errorf("failed to create endpoints: %w", err)
		}
	} else {
		eps.ResourceVersion = exist.ResourceVersion
		eps.SetOwnerReferences(mergeOwnerReferences(eps.GetOwnerReferences(), exist.GetOwnerReferences()))
		eps.SetLabels(mergeMaps(eps.Labels, exist.Labels))
		eps.SetAnnotations(mergeMaps(eps.Annotations, exist.Annotations))

		err := c.Update(ctx, eps)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return fmt.Errorf("failed to update endpoints: %w", err)
		}
	}

	return nil
}

// CreateOrUpdateStatefulSet applies the given StatefulSet against the client.
func CreateOrUpdateStatefulSet(ctx context.Context, c client.Client, ss *apps_v1.StatefulSet, l log.Logger) error {
	var exist apps_v1.StatefulSet
	err := c.Get(ctx, client.ObjectKeyFromObject(ss), &exist)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve existing statefulset: %w", err)
	}

	if k8s_errors.IsNotFound(err) {
		err := c.Create(ctx, ss)
		if err != nil {
			return fmt.Errorf("failed to create statefulset: %w", err)
		}
	} else {
		ss.ResourceVersion = exist.ResourceVersion
		ss.SetOwnerReferences(mergeOwnerReferences(ss.GetOwnerReferences(), exist.GetOwnerReferences()))
		ss.SetLabels(mergeMaps(ss.Labels, exist.Labels))
		ss.SetAnnotations(mergeMaps(ss.Annotations, exist.Annotations))

		err := c.Update(ctx, ss)
		// Statefulsets have a large number of fields that are immutable after creation,
		// so we sometimes need to delete and recreate.
		// We should be mindful when making changes to try and avoid this when possible.
		if k8s_errors.IsNotAcceptable(err) || k8s_errors.IsInvalid(err) {
			level.Error(l).Log("msg", "error updating StatefulSet. Attempting to recreate", "err", err.Error())
			// Resource version should only be set when updating
			ss.ResourceVersion = ""

			// do a quicker deletion of the old statefulset to minimize downtime before we spin up new pods
			err = c.Delete(ctx, ss, client.GracePeriodSeconds(5))
			if err != nil {
				return fmt.Errorf("failed to update statefulset when deleting old statefulset: %w", err)
			}
			err = c.Create(ctx, ss)
			if err != nil {
				return fmt.Errorf("failed to update statefulset when creating replacement statefulset: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to update statefulset: %w", err)
		}
	}

	return nil
}

// CreateOrUpdateDaemonSet applies the given DaemonSet against the client.
func CreateOrUpdateDaemonSet(ctx context.Context, c client.Client, ss *apps_v1.DaemonSet, l log.Logger) error {
	var exist apps_v1.DaemonSet
	err := c.Get(ctx, client.ObjectKeyFromObject(ss), &exist)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve existing daemonset: %w", err)
	}

	if k8s_errors.IsNotFound(err) {
		err := c.Create(ctx, ss)
		if err != nil {
			return fmt.Errorf("failed to create daemonset: %w", err)
		}
	} else {
		ss.ResourceVersion = exist.ResourceVersion
		ss.SetOwnerReferences(mergeOwnerReferences(ss.GetOwnerReferences(), exist.GetOwnerReferences()))
		ss.SetLabels(mergeMaps(ss.Labels, exist.Labels))
		ss.SetAnnotations(mergeMaps(ss.Annotations, exist.Annotations))

		err := c.Update(ctx, ss)
		if k8s_errors.IsNotAcceptable(err) || k8s_errors.IsInvalid(err) {
			level.Error(l).Log("msg", "error updating Daemonset. Attempting to recreate", "err", err.Error())
			// Resource version should only be set when updating
			ss.ResourceVersion = ""

			err = c.Delete(ctx, ss)
			if err != nil {
				return fmt.Errorf("failed to update daemonset: deleting old daemonset: %w", err)
			}
			err = c.Create(ctx, ss)
			if err != nil {
				return fmt.Errorf("failed to update daemonset: creating new deamonset: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to update daemonset: %w", err)
		}
	}

	return nil
}

// CreateOrUpdateDeployment applies the given DaemonSet against the client.
func CreateOrUpdateDeployment(ctx context.Context, c client.Client, d *apps_v1.Deployment, l log.Logger) error {
	var exist apps_v1.Deployment
	err := c.Get(ctx, client.ObjectKeyFromObject(d), &exist)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("failed to retrieve existing Deployment: %w", err)
	}

	if k8s_errors.IsNotFound(err) {
		err := c.Create(ctx, d)
		if err != nil {
			return fmt.Errorf("failed to create Deployment: %w", err)
		}
	} else {
		d.ResourceVersion = exist.ResourceVersion
		d.SetOwnerReferences(mergeOwnerReferences(d.GetOwnerReferences(), exist.GetOwnerReferences()))
		d.SetLabels(mergeMaps(d.Labels, exist.Labels))
		d.SetAnnotations(mergeMaps(d.Annotations, exist.Annotations))

		err := c.Update(ctx, d)
		if k8s_errors.IsNotAcceptable(err) || k8s_errors.IsInvalid(err) {
			level.Error(l).Log("msg", "error updating Deployment. Attempting to recreate", "err", err.Error())
			// Resource version should only be set when updating
			d.ResourceVersion = ""

			err = c.Delete(ctx, d)
			if err != nil {
				return fmt.Errorf("failed to update Deployment: deleting old Deployment: %w", err)
			}
			err = c.Create(ctx, d)
			if err != nil {
				return fmt.Errorf("failed to update Deployment: creating new Deployment: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to update Deployment: %w", err)
		}
	}

	return nil
}

func mergeOwnerReferences(new, old []meta_v1.OwnerReference) []meta_v1.OwnerReference {
	existing := make(map[types.UID]bool)
	for _, ref := range old {
		existing[ref.UID] = true
	}
	for _, ref := range new {
		if _, ok := existing[ref.UID]; !ok {
			old = append(old, ref)
		}
	}
	return old
}

func mergeMaps(new, old map[string]string) map[string]string {
	if old == nil {
		old = make(map[string]string, len(new))
	}
	for k, v := range new {
		old[k] = v
	}
	return old
}
