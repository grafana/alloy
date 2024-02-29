#!/usr/bin/env bash

ROOT=$(git rev-parse --show-toplevel)


# Generate objects and controllers for our CRDs
cd $ROOT/internal/static/operator/apis/monitoring/v1alpha1
controller-gen object paths=.
controller-gen crd:crdVersions=v1,maxDescLen=0 paths=. output:crd:dir=$ROOT/operations/agent-static-operator/crds

# Generate CRDs for prometheus-operator.
PROM_OP_DEP_NAME="github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
PROM_OP_DIR=$(go list -f '{{.Dir}}' $PROM_OP_DEP_NAME)

cd $PROM_OP_DIR
controller-gen crd:crdVersions=v1,maxDescLen=0 paths=.  output:crd:dir=$ROOT/operations/agent-static-operator/crds

# Remove known Prometheus-Operator CRDS we don't generate. (An allowlist would
# be better here, but rfratto's bash skills are bad.)
rm -f $ROOT/operations/agent-static-operator/crds/monitoring.coreos.com_alertmanagers.yaml
rm -f $ROOT/operations/agent-static-operator/crds/monitoring.coreos.com_prometheuses.yaml
rm -f $ROOT/operations/agent-static-operator/crds/monitoring.coreos.com_prometheusrules.yaml
rm -f $ROOT/operations/agent-static-operator/crds/monitoring.coreos.com_thanosrulers.yaml
