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
importer and auth configs, and four Kubernetes core types (`Volume`,
`VolumeMount`, `Quantity`, `EnvVarSource`). These used to be `$ref`s to
`raw.githubusercontent.com`. Helm resolves remote `$ref`s while validating
values, so those URLs made `helm template`/`install` fail outright without
network — an air-gapped install could not render the chart at all.

They are now referenced by **`urn:`** instead:

| URN | Replaces |
| --- | --- |
| `urn:trustify:importer` | `.../trustify/<ref>/modules/importer/schema/importer.json` |
| `urn:trustify:auth` | `.../trustify/<ref>/common/auth/schema/auth.json` |
| `urn:k8s:v1.29.0:Volume` etc. | `.../kubernetes-json-schema/.../_definitions.json#/definitions/io.k8s...` |

Helm resolves the `urn:` scheme through the pluggable `chartutil.URNResolver`.
**This repository registers nothing**, and an unresolved URN is deliberately
not an error: Helm logs a warning and substitutes a permissive schema. So
`helm install` of this chart renders offline, but the six subtrees above are
*not validated* — that is the trade-off for working without network.

A consumer that embeds the chart in a Go program can register the real schemas
and get full offline validation back; the RHTPA operator does this from its
`main.go` and keeps vendored copies of all six documents.

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
