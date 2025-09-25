# MC Controller Helm Chart

This Helm chart installs the MC Controller, a Kubernetes operator for managing MinIO resources.

## Prerequisites

- Kubernetes 1.21+
- Helm 3.0+

## Installation

### Add the Helm repository

```bash
helm repo add mc-controller https://github.com/mxcd/mc-controller/releases/latest/download/
helm repo update
```

### Install the chart

```bash
helm install mc-controller mc-controller/mc-controller --namespace mc-controller-system --create-namespace
```

### Install with custom values

```bash
helm install mc-controller mc-controller/mc-controller \
  --namespace mc-controller-system \
  --create-namespace \
  --set image.tag=v0.1.0 \
  --set resources.limits.memory=256Mi
```

## Configuration

The following table lists the configurable parameters and their default values:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of controller replicas | `1` |
| `image.repository` | Controller image repository | `ghcr.io/mxcd/mc-controller` |
| `image.tag` | Controller image tag | `""` (uses chart appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `resources.limits.cpu` | CPU resource limits | `500m` |
| `resources.limits.memory` | Memory resource limits | `128Mi` |
| `resources.requests.cpu` | CPU resource requests | `10m` |
| `resources.requests.memory` | Memory resource requests | `64Mi` |
| `serviceAccount.create` | Create service account | `true` |
| `rbac.create` | Create RBAC resources | `true` |
| `metrics.enabled` | Enable metrics endpoint | `true` |
| `metrics.port` | Metrics port | `8080` |
| `health.port` | Health probe port | `8081` |
| `leaderElection.enabled` | Enable leader election | `true` |
| `webhook.enabled` | Enable webhook server | `false` |
| `webhook.port` | Webhook server port | `9443` |

## Usage Examples

### Install with monitoring enabled

```bash
helm install mc-controller mc-controller/mc-controller \
  --namespace mc-controller-system \
  --create-namespace \
  --set metrics.serviceMonitor.enabled=true \
  --set metrics.serviceMonitor.additionalLabels.release=prometheus
```

### Install with custom resources

```bash
helm install mc-controller mc-controller/mc-controller \
  --namespace mc-controller-system \
  --create-namespace \
  --set resources.limits.cpu=1000m \
  --set resources.limits.memory=512Mi \
  --set resources.requests.cpu=100m \
  --set resources.requests.memory=128Mi
```

### Install with additional environment variables

```bash
helm install mc-controller mc-controller/mc-controller \
  --namespace mc-controller-system \
  --create-namespace \
  --set-json 'env=[{"name":"LOG_LEVEL","value":"debug"}]'
```

## Upgrading

```bash
helm upgrade mc-controller mc-controller/mc-controller --namespace mc-controller-system
```

## Uninstalling

```bash
helm uninstall mc-controller --namespace mc-controller-system
```

Note: This will not remove the CRDs automatically. To remove them:

```bash
kubectl delete crd aliases.mc-controller.mxcd.de
kubectl delete crd buckets.mc-controller.mxcd.de
kubectl delete crd endpoints.mc-controller.mxcd.de
kubectl delete crd lifecyclepolicies.mc-controller.mxcd.de
kubectl delete crd policies.mc-controller.mxcd.de
kubectl delete crd policyattachments.mc-controller.mxcd.de
kubectl delete crd users.mc-controller.mxcd.de
```

## Contributing

For information on contributing to this Helm chart, please see the main project [README](https://github.com/mxcd/mc-controller/blob/main/README.md).

## License

This Helm chart is licensed under the Apache License 2.0. See [LICENSE](https://github.com/mxcd/mc-controller/blob/main/LICENSE) for details.