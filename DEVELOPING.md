# Development instructions

> Happy when helming? Most likely not!

**NOTE:** All commands are relative to this file.

## Database initialization jobs

Both initialization jobs default to the existing PostgreSQL client implementation:

```yaml
initialization:
  mode: psql
```

To use the application binary instead, set `initialization.mode=trustd`. This
single setting changes both `createDatabase` and `createImporters`; it does not
change the migration job. The selected application image must provide
`trustd db provision` and `trustd db importers` (backend PR #2923).

With no per-job image overrides, `psql` selects `docker.io/library/postgres:17.2`
and `trustd` inherits the top-level application `image` settings. Explicit
`modules.createDatabase.image` and `modules.createImporters.image` overrides
remain supported. Clear or replace PostgreSQL image overrides when opting into
`trustd`. This also applies to old image defaults retained by `--reuse-values`;
prefer normal upgrades using the new chart defaults plus your values files.

Administrator connections still use the merged `createDatabase` and `database`
settings. The application role uses the separate `database` credentials, including
Secret references. Both implementations preserve existing data, update the
application password, and grant schema/table access. Run migrations as the same
administrator so its default privileges apply to future tables.

Importer initialization retains its database overrides and hook ordering.
`psql` executes the original SQL; `trustd` reads a JSON ConfigMap and upserts
configurations atomically using query parameters.

## Kind integration tests with chart-testing

CI uses `helm/chart-testing-action`, `helm/kind-action`, and `ct install` to test
fresh installations of both charts. Each case has its own cluster with PostgreSQL,
Keycloak and realm setup, ingress, filesystem PVCs, server, importer, and all
initialization/migration jobs. Disabled importer fixtures prevent external feed
traffic; optional observability services remain disabled.

The matrix uses `ghcr.io/guacsec/trustd:latest` plus the `.z` aliases for the two
highest stable minor release lines (currently `0.7.z` and `0.6.z`). All three run
with the default `psql` mode; `latest` additionally runs with `trustd`. Tags are
resolved once per workflow and the actual digests are recorded in its summary.
The native case fails explicitly until `latest` contains both new commands;
there is no fallback or silently skipped native coverage.

The application chart's optional Helm test hooks (`tests.enabled=true`) check
unauthenticated rejection, Keycloak client credentials, the seeded importer,
and SBOM upload/lookup/download. They use the application Service; separate
workflow checks exercise Keycloak and application ingress routing.

### Run locally

Requirements: Docker, Kind 0.30.0, kubectl 1.34.0, Helm 3.22.1, chart-testing
3.14.0, and Python 3. The commands below use Linux Docker networking, matching CI.

```shell
kind create cluster --name chart-testing --image kindest/node:v1.34.0

docker run --detach --name kind-cloud-provider --network host \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  registry.k8s.io/cloud-provider-kind/cloud-controller-manager:v0.9.0

kubectl --context kind-chart-testing label nodes --all \
  node.kubernetes.io/exclude-from-external-load-balancers-
kubectl --context kind-chart-testing create namespace trustify-ci
ct install --config .github/ct-install.yaml --charts charts/trustify-infrastructure \
  --namespace trustify-ci --release-name infrastructure
python3 hack/kind-ingress.py sso

# Existing migration hooks require this PVC before Helm's normal resource phase.
helm template trustify charts/trustify --namespace trustify-ci \
  --values charts/trustify/ci/kind-values.yaml \
  --show-only templates/services/server/010-PersistentVolumeClaim-storage.yaml \
  | kubectl --context kind-chart-testing -n trustify-ci apply -f -
kubectl --context kind-chart-testing -n trustify-ci label pvc storage \
  app.kubernetes.io/managed-by=Helm --overwrite
kubectl --context kind-chart-testing -n trustify-ci annotate pvc storage \
  meta.helm.sh/release-name=trustify meta.helm.sh/release-namespace=trustify-ci --overwrite

ct install --config .github/ct-install.yaml --charts charts/trustify \
  --namespace trustify-ci --release-name trustify \
  --helm-extra-set-args '--set-string image.fullName=ghcr.io/guacsec/trustd:0.7.z'
python3 hack/kind-ingress.py application
```

For native initialization, use a suitable `latest` image and append
`--set initialization.mode=trustd` inside `--helm-extra-set-args`.
Each chart's `ci/kind-values.yaml` supplies the CI fixtures automatically.
The PVC bootstrap uses the actual chart template and Helm ownership metadata;
Helm adopts it during installation. This is required by the existing filesystem
migration pre-install hook and is not a change to production PVC lifecycle.

Installation arguments are kept separate from `helm-extra-args`, since `ct`
also passes the latter to `helm test`. Upgrade tests are intentionally not enabled.

On failure, run `bash hack/kind-diagnostics.sh`; inspect `kind-artifacts/` for
resource state, hook/workload logs, events, ingress-provider logs, and Helm status.
Generated Secrets and Helm values are excluded and fixture credentials are
redacted. CI uploads these artifacts before Kind's cleanup action deletes the
cluster. After a local run, clean up explicitly:

```shell
docker rm -f kind-cloud-provider
kind delete cluster --name chart-testing
```

## Updating chart dependencies

```shell
helm dependency update charts/trustify-infrastructure
```

## Updating the JSON schema

The JSON schema must cover all values in the values file used by the Helm chart. The only exception are sections in
the values which are intended to be used by other charts, re-using the same values file. An example of this is the
`.global` section.

Unfortunately, Helm requires the JSON schema to be authored in JSON. To make that a little bit easier, we author it
in YAML and then convert it to JSON. For example, using:

```shell
python3 -c 'import sys, yaml, json; print(json.dumps(yaml.safe_load(sys.stdin), indent=2))' < charts/trustify/values.schema.yaml > charts/trustify/values.schema.json
```

It is also possible to use `yq` for this:

```shell
yq eval -o=json charts/trustify/values.schema.yaml > charts/trustify/values.schema.json
```

### External schema references

The values schema needs definitions owned by other projects: the trustify
importer and auth configs, and five Kubernetes core types (`Volume`,
`VolumeMount`, `Quantity`, `EnvVar`, `EnvVarSource`). These used to be `$ref`s to
`raw.githubusercontent.com`. Helm resolves remote `$ref`s while validating
values, so those URLs made `helm template`/`install` fail outright without
network — an air-gapped install could not render the chart at all.

They are now referenced by **`urn:`** instead:

| URN | Replaces |
| --- | --- |
| `urn:trustify:importer` | `.../trustify/<ref>/modules/importer/schema/importer.json` |
| `urn:trustify:auth` | `.../trustify/<ref>/common/auth/schema/auth.json` |
| `urn:k8s:v1.29.0:Volume` etc. | `.../kubernetes-json-schema/.../_definitions.json#/definitions/io.k8s...` |

Helm resolves the `urn:` scheme through the pluggable `chartutil.URNResolver`,
a Go variable with no CLI flag or environment variable behind it. An
unresolved URN is deliberately not an error: Helm logs a warning and
substitutes a permissive schema. So `helm install` of this chart renders
offline, but the `helm` CLI does **not** validate the seven subtrees above.

Nothing is inlined into the chart to fix that, on purpose: the RHTPA operator
ships its own copy of this chart and the two must stay identical, so the
schema file cannot carry anything specific to either side. The `urn:` spelling
is version-agnostic, which is what lets the two copies match even though they
resolve different trustify releases — the operator registers its own vendored
copies from `main.go`, and gets full offline validation that way.

### Validating the values files in CI

Since the CLI cannot check those subtrees, a small Go test does it instead:

```shell
cd hack/schema-validator && go test ./...
```

It registers the schemas vendored in `hack/schemas/` as Helm's URN resolver
and validates every values-file combination the README documents —
`values-minikube.yaml`, `values-ocp-no-aws.yaml`, `values-ocp-aws.yaml`,
`values-ocp-no-aws.yaml + values-crc.yaml`, and each base plus
`values-importers.yaml`. The importer cases are the ones that actually
exercise `urn:trustify:importer`. **Add a case when you add a values file**,
or it will not be covered.

The suite also asserts that registering the schemas genuinely tightens
validation, so the mechanism cannot go inert unnoticed, and that an
unregistered URN still degrades to permissive rather than failing — the
property a plain offline `helm install` depends on.

Refreshing the vendored copies:

```shell
hack/vendor-values-schemas.py            # verify digests and chart refs
hack/vendor-values-schemas.py --fetch    # re-download from upstream
```

Their URLs and SHA-256 digests live in `hack/schemas/SOURCES.json`; to bump a
version, edit `upstream` there and run `--fetch`. The `--check` mode runs in
CI and also fails if a remote `$ref` creeps back into the chart or a URN is
referenced that nothing vendors.

Two constraints when editing these refs:

- **A URN must not carry a JSON-pointer fragment.** The fallback for an
  unresolved URN is a bare `true` schema, and `#/definitions/X` cannot resolve
  against it, which turns graceful degradation into a hard failure. Any
  Kubernetes type referenced this way has to be a whole self-contained
  document, not a pointer into `_definitions.json`.
- **This requires Helm >= 3.20.** Older Helm has no loader for the `urn:`
  scheme and fails with `Reference urn:... must be canonical`.

## Linting Helm charts

```shell
helm lint ./charts/trustify --values values-minikube.yaml --set-string appDomain=.localhost
```

Quick lint even more:

```shell
ct lint --charts ./charts/trustify --helm-lint-extra-args "--values values-minikube.yaml --set-string appDomain=.localhost"
```

Lint like CI:

```shell
ct lint --config .github/ct.yaml --all
 ```

## Unit testing Helm templates

Install the [helm-unittest](https://github.com/helm-unittest/helm-unittest) plugin (one-time setup):

```shell
helm plugin install https://github.com/helm-unittest/helm-unittest --version v1.1.1 --verify=false
```

Run the tests:

```shell
helm unittest charts/trustify
```

Test files live in `charts/trustify/tests/` with the suffix `_test.yaml`.

## Find that whitespace

```shell
helm template --debug charts/trustify
```
