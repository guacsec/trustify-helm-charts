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
helm upgrade --install --dependency-update -n $NAMESPACE infrastructure charts/trustify-infrastructure --values values-minikube.yaml --set-string keycloak.ingress.hostname=sso$APP_DOMAIN --set-string appDomain=$APP_DOMAIN --set jaeger.enabled=true --set-string jaeger.allInOne.ingress.hosts[0]=jaeger$APP_DOMAIN --set tracing.enabled=true --set prometheus.enabled=true --set-string prometheus.server.ingress.hosts[0]=prometheus$APP_DOMAIN --set metrics.enabled=true
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
helm repo add trustify https://trustification.io/trustify-helm-charts/
```

And then, modify any of the previous `helm` commands to use:

```bash
helm […] --devel trustify/<chart> […]
```

## Initial set of importers

You can create an initial set of importers by adding the values file `values-importers.yaml`.
