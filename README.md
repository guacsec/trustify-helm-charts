# Deploying Trustify using Helm

## From a local checkout

From a local copy of the repository, execute one of the following deployments.

### Minikube

Create a new cluster:

```bash
minikube start --cpus 8 --memory 24576 --disk-size 20gb --addons ingress,dashboard
```

Create a new namespace:

```bash
kubectl create ns trustify
```

Use it as default:

```bash
kubectl config set-context --current --namespace=trustify
```

Evaluate the application domain and namespace:

```bash
NAMESPACE=trustify
APP_DOMAIN=.$(minikube ip).nip.io
```

Install the infrastructure services:

```bash
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure --values values-minikube.yaml --set-string keycloak.ingress.hostname=sso$APP_DOMAIN --set-string appDomain=$APP_DOMAIN
```

Then deploy the application:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-minikube.yaml --set-string appDomain=$APP_DOMAIN
```

#### Enable tracing and metrics

```bash
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure --values values-minikube.yaml --set-string keycloak.ingress.hostname=sso$APP_DOMAIN --set-string appDomain=$APP_DOMAIN --set jaeger.enabled=true --set-string jaeger.allInOne.ingress.host=jaeger$APP_DOMAIN --set tracing.enabled=true --set prometheus.enabled=true --set-string prometheus.server.ingress.host=prometheus$APP_DOMAIN --set metrics.enabled=true
```

Using the default http://infrastructure-otelcol:4317 OpenTelemetry collector endpoint. This works with the previous
command of the default infrastructure deployment:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-minikube.yaml --set-string appDomain=$APP_DOMAIN --set tracing.enabled=true --set metrics.enabled=true
```

Setting an explicit OpenTelemetry collector endpoint:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-minikube.yaml --set-string appDomain=$APP_DOMAIN --set tracing.enabled=true --set metrics.enabled=true --set-string collector.endpoint="http://infrastructure-otelcol:4317"
```

### Kind

Create a new cluster:

```bash
kind create cluster --config kind/config.yaml
kubectl apply -f https://kind.sigs.k8s.io/examples/ingress/deploy-ingress-nginx.yaml
```

The rest works like the `minikube` approach. The `APP_DOMAIN` is different though:

```bash
APP_DOMAIN=.$(kubectl get node kind-control-plane -o jsonpath='{.status.addresses[?(@.type == "InternalIP")].address}' | awk '// { print $1 }').nip.io
```

#### Important Note for Kind + Podman Users (macOS/Windows)

When using Kind with Podman (instead of Docker Desktop), the Kind cluster runs inside a VM managed by Podman. This creates a networking scenario where:

- **Pod's localhost** = the pod itself (not the VM, not the host)
- **VM localhost** = the VM (not the host)
- **Host localhost** = your actual machine

For proper networking, you need to use `APP_DOMAIN=.127.0.0.1.nip.io` and patch the CoreDNS configuration:

1. **Get the ingress controller ClusterIP:**
   ```bash
   kubectl -n ingress-nginx get svc ingress-nginx-controller -o jsonpath='{.spec.clusterIP}'
   ```

2. **Patch the CoreDNS configuration:**
   ```bash
   kubectl -n kube-system get configmap coredns -o yaml > coredns-config.yaml
   ```

   Edit the `coredns-config.yaml` file and add the following to the `Corefile` section:
   ```yaml
   hosts {
       10.96.75.197 sso.127.0.0.1.nip.io  # Replace 10.96.75.197 with your actual ClusterIP
       fallthrough
   }
   ```

   Apply the updated configuration:
   ```bash
   kubectl -n kube-system apply -f coredns-config.yaml
   ```

3. **Restart CoreDNS:**
   ```bash
   kubectl -n kube-system rollout restart deployment coredns
   ```

This ensures that DNS resolution works correctly within the Kind cluster when using Podman.

### CRC

Create a new cluster:

```bash
crc start --cpus 8 --memory 32768 --disk-size 80
```

Create a new namespace:

```bash
oc new-project trustify
```

Evaluate the application domain and namespace:

```bash
NAMESPACE=trustify
APP_DOMAIN=-$NAMESPACE.$(oc -n openshift-ingress-operator get ingresscontrollers.operator.openshift.io default -o jsonpath='{.status.domain}')
```

Provide the trust anchor:

```bash
oc get secret -n openshift-ingress router-certs-default -o go-template='{{index .data "tls.crt"}}' | base64 -d > tls.crt
oc create configmap crc-trust-anchor --from-file=tls.crt -n trustify
rm tls.crt
```

Deploy the infrastructure:

```bash
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure --values values-ocp-no-aws.yaml --set-string keycloak.ingress.hostname=sso$APP_DOMAIN --set-string appDomain=$APP_DOMAIN
```

Deploy the application:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-ocp-no-aws.yaml --set-string appDomain=$APP_DOMAIN --values values-crc.yaml
```

## OpenShift with AWS resources

Instead of using Keycloak and the filesystem storage, it is also possible to use AWS Cognito and S3.

Deploy only the application:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-ocp-aws.yaml --set-string appDomain=$APP_DOMAIN
```

## From a released chart

Instead of using a local checkout, you can also use a released chart.

> [!NOTE]
> You will still need a "values" files, providing the necessary information. If you don't clone the repository, you'll
> need to create a value yourself.

For this, you will need to add the following repository:

```bash
helm repo add trustify https://guacsec.github.io/trustify-helm-charts/
```

And then, modify any of the previous `helm` commands to use:

```bash
helm […] --devel trustify/<chart> […]
```

## Ingress Configuration

The Trustify Helm chart supports flexible ingress configuration with multiple hostnames and automatic TLS setup.

### Default Behavior

By default, the chart creates an ingress with a single hostname using the pattern `{service-name}{appDomain}`:

```yaml
# Default configuration
appDomain: .example.com
# Results in hostname: server.example.com
```

### Multiple Hostnames

You can configure multiple hostnames for the same service:

```yaml
ingress:
  hosts:
    - "api.trustify.com"
    - "api-staging.trustify.com"
    - "api-dev.trustify.com"
```

### Custom TLS Configuration

For advanced scenarios, you can provide explicit TLS configuration:

```yaml
ingress:
  hosts:
    - "api.trustify.com"
    - "api-staging.trustify.com"
  tls:
    - hosts:
        - "api.trustify.com"
        - "api-staging.trustify.com"
      secretName: "custom-tls"
```

### Multiple TLS Blocks

You can configure different TLS secrets for different host groups:

```yaml
ingress:
  hosts:
    - "api.trustify.com"
    - "api-staging.trustify.com"
    - "api-dev.trustify.com"
  tls:
    - hosts:
        - "api.trustify.com"
        - "api-staging.trustify.com"
      secretName: "prod-tls"
    - hosts:
        - "api-dev.trustify.com"
      secretName: "dev-tls"
```

### Module-level Overrides

You can override global ingress settings at the module level:

```yaml
ingress:
  hosts:
    - "default.example.com"
modules:
  server:
    ingress:
      hosts:
        - "api.trustify.com"
        - "api-staging.trustify.com"
```

### Ingress Class and Annotations

Configure ingress class and additional annotations:

```yaml
ingress:
  className: "nginx"
  additionalAnnotations:
    nginx.ingress.kubernetes.io/rate-limit: "100"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
```

### Disable Ingress

You can disable ingress creation globally or per module:

```yaml
# Disable globally
ingress:
  enabled: false

# Disable per module
modules:
  server:
    enabled: false
```

## Proxy Testing (QE)

The `values-proxy-test.yaml` file supports QE validation of git-based importer proxy
auto-detection (TC-5174). It deploys a Squid forward proxy in-cluster and injects
`HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` into the importer pod via the `extraEnv` mechanism.

### Deploy the proxy and network policy

Apply these **before** deploying Trustify to avoid a race condition where the importer
could reach github.com directly before the NetworkPolicy propagates:

```bash
oc apply -f squid-proxy.yaml -n $NAMESPACE
oc apply -f networkpolicy-block-importer-egress.yaml -n $NAMESPACE
```

### Deploy with proxy test values

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify \
  --values values-ocp-no-aws.yaml \
  --values values-proxy-test.yaml \
  --set-string appDomain=$APP_DOMAIN \
  --set image.fullName="<image-reference>"
```

### NO_PROXY configuration

The default `NO_PROXY` in `values-proxy-test.yaml` contains only the minimal
common baseline. You must extend it with infrastructure-specific hostnames to
prevent internal traffic from routing through Squid.

**OCP-only (bundled Keycloak + PostgreSQL):**

Add the bundled service names and the Keycloak route FQDN:

```bash
--set 'modules.importer.extraEnv[2].name=NO_PROXY' \
--set 'modules.importer.extraEnv[2].value=localhost,127.0.0.1,.svc,.cluster.local,trustify-server,infrastructure-postgresql,infrastructure-keycloak,sso-$NAMESPACE.$APP_DOMAIN'
```

**AWS (RDS + Cognito + S3):**

Add the RDS endpoint hostname and the Cognito issuer hostname:

```bash
--set 'modules.importer.extraEnv[2].name=NO_PROXY' \
--set 'modules.importer.extraEnv[2].value=localhost,127.0.0.1,.svc,.cluster.local,trustify-server,<RDS-endpoint>,<Cognito-issuer-hostname>'
```

For example:

```bash
--set 'modules.importer.extraEnv[2].value=localhost,127.0.0.1,.svc,.cluster.local,trustify-server,trustify-db.abc123.us-east-1.rds.amazonaws.com,cognito-idp.us-east-1.amazonaws.com'
```

| Infrastructure | Hostnames to add to NO_PROXY |
|---|---|
| Bundled PostgreSQL | `infrastructure-postgresql` |
| Bundled Keycloak | `infrastructure-keycloak`, `sso-<namespace>.<appDomain>` |
| AWS RDS | RDS endpoint (e.g., `trustify-db.abc123.us-east-1.rds.amazonaws.com`) |
| AWS Cognito | Cognito issuer host (e.g., `cognito-idp.us-east-1.amazonaws.com`) |
| AWS S3 | Not needed — S3 traffic should go through the proxy |

## Initial set of importers

You can create an initial set of importers by adding the values file `values-importers.yaml`.

