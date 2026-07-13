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
