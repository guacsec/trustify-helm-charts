# Coding Conventions

## Language and Framework

- Kubernetes Helm charts (YAML + Go templates)
- Helm 3 (v3.13.3+ used in CI)
- Two charts: `trustify` (main application) and `trustify-infrastructure` (supporting services like Keycloak)
- Values schema defined in YAML (`values.schema.yaml`) and auto-generated to JSON (`values.schema.json`)

## Code Style

- YAML: 2-space indentation
- Go template helpers: wrap each definition in `{{/* comment */}}` documenting arguments
- Use `nindent` for indentation in template includes (not `indent`)
- Ensure JSON schema is regenerated from YAML after schema changes: `python3 -c 'import sys, yaml, json; print(json.dumps(yaml.safe_load(sys.stdin), indent=2))' < charts/trustify/values.schema.yaml > charts/trustify/values.schema.json`

## Naming Conventions

- Template helpers: `trustification.<domain>.<function>` (e.g., `trustification.common.name`, `trustification.oidc.frontendIssuerUrl`)
- Helper files: `_<domain>.tpl` prefixed with underscore (e.g., `_common.tpl`, `_postgres.tpl`)
- Kubernetes resources: numbered prefix `NNN-Kind.yaml` (e.g., `010-ConfigMap.yaml`, `020-Job.yaml`, `030-Deployment.yaml`)
- Module variables: `$mod := dict "root" . "name" "<name>" "component" "<component>" "module" .Values.modules.<name>`
- Values keys: camelCase (e.g., `appDomain`, `syncInterval`, `uploadLimit`)

## File Organization

- `charts/trustify/` — main chart
  - `templates/helpers/` — reusable Go template helper definitions (`_*.tpl`)
  - `templates/services/<service-name>/` — per-service Kubernetes manifests (numbered)
  - `templates/init/<init-task>/` — init job manifests (numbered)
  - `values.yaml` — default values
  - `values.schema.yaml` — schema source (YAML)
  - `values.schema.json` — auto-generated schema (JSON)
- `charts/trustify-infrastructure/` — infrastructure chart (Keycloak setup)
- `values-*.yaml` — environment-specific value overrides at repo root (minikube, CRC, OCP)
- `kind/` — Kind cluster configuration for local testing
- `.github/` — CI workflows and chart-testing configuration

## Error Handling

- Use `required` function for mandatory values: `{{- $_ := required .msg .value }}`
- Use `default` chains for optional values with fallbacks: `(.module.index).syncInterval | default .storage.syncInterval | default .root.Values.index.syncInterval | default "1800s"`
- Guard optional sections with `{{- with ... }}` or `{{- if ... }}`
- Guard entire resource files with module enable checks: `{{- if .Values.modules.<name>.enabled }}`

## Testing Conventions

- Chart linting via `ct lint` (chart-testing tool)
- CI checks for uncommitted schema changes
- Environment-specific testing with `values-minikube.yaml`
- Kind cluster for integration testing in CI

## Commit Messages

- Conventional Commits format: `type(scope): description`
- Types used: `feat`, `fix`, `chore`, `doc`
- Scope is optional and domain-specific (e.g., `auth`)

## Shared Modules and Reuse

- `charts/trustify/templates/helpers/` — all reusable template helpers
  - `_common.tpl` — env var value helpers, byte size formatting
  - `_deployment.tpl` — deployment-level patterns (replicas, pod spec, containers)
  - `_labels.tpl` — standard Kubernetes labels and selector labels
  - `_name.tpl` — resource naming
  - `_annotations.tpl` — common annotation patterns
  - `_postgres.tpl` — PostgreSQL connection env vars
  - `_storage.tpl` — storage configuration (volumes, mounts, env vars)
  - `_oidc.tpl` — OIDC/auth configuration
  - `_image.tpl` — container image references
  - `_infrastructure.tpl` — infrastructure probes and ports
  - `_ingress.tpl` — Ingress resource patterns
  - `_tls.tpl` — TLS configuration
  - `_http.tpl` — HTTP server configuration
  - `_rust.tpl` — Rust-specific env vars
  - `_extra.tpl` — extra volumes, mounts, and env vars (user-extensible)
- `charts/trustify-infrastructure/templates/_helper.tpl` — infrastructure chart helpers

## Documentation

- `README.md` — project overview and usage instructions
- `DEVELOPING.md` — development setup and workflow
- `charts/trustify/templates/NOTES.txt` — post-install notes shown by Helm

## Dependencies

- Chart dependencies managed via `Chart.yaml`
- External chart repos: Jaeger, Prometheus community, OpenTelemetry
- Schema validation uses Yamale and yamllint (Python-based, pulled in CI)

## Performance Optimization

- No specific performance conventions documented yet
