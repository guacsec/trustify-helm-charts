#!/usr/bin/env bash
# Collect troubleshooting artifacts before the Kind test cluster is deleted.
# Usage from the repository root: bash hack/kind-diagnostics.sh
# Requires kubectl and Helm, with Helm targeting the kind-chart-testing cluster.
# CI runs this after installation/tests even when an earlier step fails.
#
# Write workload/resource state, events, pod descriptions, Helm release status,
# application manifests, current/previous pod logs, and NGINX controller logs to
# kind-artifacts/. Collection continues after command failures so a partially
# installed cluster still yields useful diagnostics. CI uploads this directory.
#
# Avoid Secret resources, Helm values, and infrastructure manifests containing
# generated credentials; redact the known fixture credentials from most output.
# NGINX logs are collected verbatim. This is intended for the disposable CI
# fixtures, not for exporting diagnostics from a production cluster.
set -u
mkdir -p kind-artifacts
redact() {
  sed -e 's/kind-test-secret/[REDACTED]/g' \
      -e 's/kind-app-password/[REDACTED]/g' \
      -e 's/kind-admin-password/[REDACTED]/g'
}
kubectl --context kind-chart-testing -n trustify-ci get pods,deployments,statefulsets,services,jobs,pvc,ingresses -o yaml \
  2>&1 | redact > kind-artifacts/resources.yaml
kubectl --context kind-chart-testing -n trustify-ci get events --sort-by=.lastTimestamp \
  2>&1 | redact > kind-artifacts/events.txt
kubectl --context kind-chart-testing -n trustify-ci describe pods \
  2>&1 | redact > kind-artifacts/pods.txt
for release in $(helm list -n trustify-ci -q); do
  helm status "$release" -n trustify-ci \
    2>&1 | redact > "kind-artifacts/$release-status.txt"
done
# The application chart has no Secret resources; do not export the infrastructure
# chart's rendered manifests, which contain generated PostgreSQL credentials.
for release in $(helm list -n trustify-ci --filter '^trustify-' -q); do
  helm get manifest "$release" -n trustify-ci \
    2>&1 | redact > kind-artifacts/application.yaml
done
for pod in $(kubectl --context kind-chart-testing -n trustify-ci get pods -o name 2>/dev/null); do
  name=${pod#pod/}
  kubectl --context kind-chart-testing -n trustify-ci logs "$pod" --all-containers --prefix --tail=2000 \
    2>&1 | redact > "kind-artifacts/$name.log"
  kubectl --context kind-chart-testing -n trustify-ci logs "$pod" --all-containers --prefix --previous --tail=500 \
    2>&1 | redact > "kind-artifacts/$name-previous.log"
done
kubectl --context kind-chart-testing -n nginx-ingress logs \
  -l app.kubernetes.io/instance=nginx --all-containers --tail=2000 \
  > kind-artifacts/nginx-ingress.log 2>&1 || true
