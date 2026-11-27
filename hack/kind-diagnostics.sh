#!/usr/bin/env bash
# Deliberately avoid Secrets, Helm values, and infrastructure manifests.
set -u
mkdir -p kind-artifacts
redact() {
  sed -e 's/kind-test-secret/[REDACTED]/g' \
      -e 's/kind-app-password/[REDACTED]/g' \
      -e 's/kind-admin-password/[REDACTED]/g'
}
kubectl --context kind-chart-testing -n trustify-ci get pods,deployments,statefulsets,jobs,pvc,ingresses -o yaml \
  2>&1 | redact > kind-artifacts/resources.yaml
kubectl --context kind-chart-testing -n trustify-ci get events --sort-by=.lastTimestamp \
  2>&1 | redact > kind-artifacts/events.txt
kubectl --context kind-chart-testing -n trustify-ci describe pods \
  2>&1 | redact > kind-artifacts/pods.txt
for release in infrastructure trustify; do
  helm status "$release" -n trustify-ci \
    2>&1 | redact > "kind-artifacts/$release-status.txt"
done
# The application chart has no Secret resources; do not export the infrastructure
# chart's rendered manifests, which contain generated PostgreSQL credentials.
helm get manifest trustify -n trustify-ci \
  2>&1 | redact > kind-artifacts/application.yaml
for pod in $(kubectl --context kind-chart-testing -n trustify-ci get pods -o name 2>/dev/null); do
  name=${pod#pod/}
  kubectl --context kind-chart-testing -n trustify-ci logs "$pod" --all-containers --prefix --tail=2000 \
    2>&1 | redact > "kind-artifacts/$name.log"
  kubectl --context kind-chart-testing -n trustify-ci logs "$pod" --all-containers --prefix --previous --tail=500 \
    2>&1 | redact > "kind-artifacts/$name-previous.log"
done
docker logs kind-cloud-provider > kind-artifacts/ingress-provider.log 2>&1 || true
