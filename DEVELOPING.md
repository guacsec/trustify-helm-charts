# Development instructions

> Happy when helming? Most likely not!

**NOTE:** All commands are relative to this file.

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

## End-to-end testing on minikube

`ct lint` and `helm unittest` only tell you the charts *render*. The
`E2E Minikube` workflow (`.github/workflows/e2e-minikube.yaml`) installs both
charts on a real cluster, exactly the way [README.md](README.md#minikube)
documents the minikube install, and then checks that the result works: the
init jobs completed, the workloads are available and did not restart, the
server answers over the ingress the chart created, and the OIDC issuer the
chart configured resolves to the realm the infrastructure chart created.

It is deliberately an install of the documented values files, not of a
purpose-built test configuration — a hook ordering mistake, a wrong service
port or a broken issuer URL shows up here and nowhere else.

It layers one extra values file over `values-minikube.yaml`:

```shell
helm upgrade --install -n trustify trustify charts/trustify \
  --values values-minikube.yaml \
  --values values-e2e-minikube.yaml \
  --set-string appDomain=".$(minikube ip).nip.io"
```

`values-e2e-minikube.yaml` only shrinks resource requests. The chart defaults
ask for 1 CPU and 8 GiB each for the server and the importer, which a CI runner
does not have — nothing else about the install differs, deliberately, because
the point is to test what the README tells people to run.

The image is the chart's own default. Running the workflow by hand
(`workflow_dispatch`) takes an optional `appImage` to deploy a different build
instead, which is also the escape hatch if the default ever moves to a registry
that needs a pull secret.

### The storage PVC has to exist before the install

The workflow creates the `storage` PVC itself, before installing the chart.
This is a workaround, and the one thing it does that the README does not.

`migrate-db` is a `pre-install` hook and mounts that PVC, but the chart renders
the PVC as an ordinary resource — which Helm creates only *after* its
pre-install hooks have completed. With `storage.type: filesystem` and no PVC
there yet, the hook pod is unschedulable (`persistentvolumeclaim "storage" not
found`) and the install blocks until it times out. It is not specific to
minikube: any greenfield `helm install` of the chart with filesystem storage
hits it, including `values-ocp-no-aws.yaml`.

Creating the PVC up front breaks the cycle; the workflow renders it from the
chart and adds the release ownership annotations so Helm adopts it rather than
failing with `invalid ownership metadata`. Once the chart creates the PVC
before the hook that needs it, drop the step.

### Reproducing locally

Two things are worth knowing:

- Wait for the ingress controller before installing. The `ingress` addon
  reports itself enabled before its admission webhook is serving, and both
  charts create `Ingress` objects; installing in between fails with
  `dial tcp ...:443: connect: connection refused`.
- The **rootless** podman driver cannot run it. ingress-nginx needs host
  ports, and `portmap` cannot set up the iptables `nat` table in a rootless
  network namespace. Use the docker driver, or rootful podman.



## Find that whitespace

```shell
helm template --debug charts/trustify
```
