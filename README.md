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
helm repo add trustify https://trustification.io/trustify-helm-charts/
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

### Automatic TLS Configuration

When you specify hostnames, TLS is automatically configured using the same hosts:

```yaml
ingress:
  hosts:
    - "api.trustify.com"
    - "api-staging.trustify.com"
# Automatically generates TLS with secret name: server-tls
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

## Initial set of importers

You can create an initial set of importers by adding the values file `values-importers.yaml`.
