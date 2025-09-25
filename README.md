# Deploying Trustify using Helm

## From a local checkout

From a local copy of the repository, execute one of the following deployments.

## Configuration Files

The repository includes several configuration files for different deployment scenarios:

### Infrastructure Configuration Files

- **`values-infrastructure-collector.yaml`** - Use when deploying with OpenTelemetry collector
  - Prometheus scrapes from `trustify-collector:8889`
  - Centralized telemetry collection
  - No direct service scraping

- **`values-infrastructure-direct.yaml`** - Use when deploying without collector
  - Prometheus scrapes directly from services (`trustify-server:9010`, `trustify-importer:9010`)
  - Requires `metrics.enabled=true` on services
  - Direct service scraping

### Application Configuration Files

- **`values-minikube.yaml`** - Minikube-specific configuration
- **`values-crc.yaml`** - CRC-specific configuration  
- **`values-ocp-aws.yaml`** - OpenShift on AWS configuration
- **`values-ocp-no-aws.yaml`** - OpenShift without AWS configuration
- **`values-collector-prometheus.yaml`** - Collector with Prometheus configuration
- **`values-importers.yaml`** - Importer-specific configuration

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

#### Enable Tracing and Metrics

You have three main options for enabling observability in your Trustify deployment:

##### Option 1: Infrastructure Collector

Deploy the infrastructure with Jaeger and Prometheus, using the infrastructure OpenTelemetry collector:

```bash
# Deploy infrastructure with observability
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure \
  --values values-minikube.yaml \
  --set-string keycloak.ingress.hostname=sso$APP_DOMAIN \
  --set-string appDomain=$APP_DOMAIN \
  --set jaeger.enabled=true \
  --set-string jaeger.allInOne.ingress.host=jaeger$APP_DOMAIN \
  --set tracing.enabled=true \
  --set prometheus.enabled=true \
  --set-string prometheus.server.ingress.host=prometheus$APP_DOMAIN \
  --set metrics.enabled=true

# Deploy application with tracing and metrics
helm upgrade --install -n $NAMESPACE trustify charts/trustify \
  --values values-minikube.yaml \
  --set-string appDomain=$APP_DOMAIN \
  --set tracing.enabled=true \
  --set metrics.enabled=true
```

##### Option 2: Direct Service Scraping

Deploy Prometheus that scrapes directly from services (requires metrics enabled on services):

```bash
# Deploy infrastructure with direct scraping
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure \
  --values values-minikube.yaml \
  --values values-infrastructure-direct.yaml \
  --set-string keycloak.ingress.hostname=sso$APP_DOMAIN \
  --set-string appDomain=$APP_DOMAIN

# Deploy application with metrics enabled
helm upgrade --install -n $NAMESPACE trustify charts/trustify \
  --values values-minikube.yaml \
  --set-string appDomain=$APP_DOMAIN \
  --set tracing.enabled=true \
  --set metrics.enabled=true
```

**Note:** This approach requires services to expose Prometheus metrics endpoints. If metrics are disabled on services, you'll see scraping errors in Prometheus.

##### Option 3: Application Collector with Prometheus (Recommended)

Deploy both the application collector and Prometheus together for centralized telemetry collection:

```bash
# Deploy infrastructure with Prometheus configured for collector
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure \
  --values values-minikube.yaml \
  --values values-infrastructure-collector.yaml \
  --set-string keycloak.ingress.hostname=sso$APP_DOMAIN \
  --set-string appDomain=$APP_DOMAIN \
  --set-string "prometheus.server.ingress.hosts[0]=prometheus$APP_DOMAIN"

# Deploy application with built-in collector
helm upgrade --install -n $NAMESPACE trustify charts/trustify \
  --values values-minikube.yaml \
  --set-string appDomain=$APP_DOMAIN \
  --set tracing.enabled=true \
  --set metrics.enabled=true \
  --set modules.collector.enabled=true
```

**Note:** This approach is recommended for deployments where the infrastructure is already provided and it is not possible to do direct scraping.

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

#### Enable OpenTelemetry Collector with CRC

To use the built-in OpenTelemetry collector:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-ocp-no-aws.yaml --set-string appDomain=$APP_DOMAIN --values values-crc.yaml --set tracing.enabled=true --set metrics.enabled=true --set modules.collector.enabled=true
```

#### Enable OpenTelemetry Collector with Prometheus on CRC

To deploy both the collector and Prometheus together:

1. First, deploy the infrastructure with Prometheus configured to scrape from the collector:

```bash
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure --values values-ocp-no-aws.yaml --values values-infrastructure-collector.yaml --set-string keycloak.ingress.hostname=sso$APP_DOMAIN --set-string appDomain=$APP_DOMAIN
```

2. Then deploy the application with the collector enabled:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-ocp-no-aws.yaml --set-string appDomain=$APP_DOMAIN --values values-crc.yaml --set tracing.enabled=true --set metrics.enabled=true --set modules.collector.enabled=true
```

## OpenShift with AWS resources

Instead of using Keycloak and the filesystem storage, it is also possible to use AWS Cognito and S3.

Deploy only the application:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-ocp-aws.yaml --set-string appDomain=$APP_DOMAIN
```

#### Enable OpenTelemetry Collector with AWS

To use the built-in OpenTelemetry collector:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-ocp-aws.yaml --set-string appDomain=$APP_DOMAIN --set tracing.enabled=true --set metrics.enabled=true --set modules.collector.enabled=true
```

#### Enable OpenTelemetry Collector with Prometheus on AWS

To deploy both the collector and Prometheus together:

1. First, deploy the infrastructure with Prometheus configured to scrape from the collector:

```bash
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure --values values-ocp-aws.yaml --values values-infrastructure-collector.yaml --set-string appDomain=$APP_DOMAIN
```

2. Then deploy the application with the collector enabled:

```bash
helm upgrade --install -n $NAMESPACE trustify charts/trustify --values values-ocp-aws.yaml --set-string appDomain=$APP_DOMAIN --set tracing.enabled=true --set metrics.enabled=true --set modules.collector.enabled=true
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
