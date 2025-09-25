# MinIO Alias CRD

The `Alias` Custom Resource Definition (CRD) provides a centralized way to manage MinIO server connection configurations, similar to the `mc alias` command in the MinIO client.

## Overview

The Alias CRD allows you to:
- Define reusable connection configurations to MinIO instances
- Centralize credential management through Kubernetes secrets
- Enable health monitoring of MinIO connections
- Reference aliases from other MinIO resources (Buckets, Users, etc.)

## Features

- **Connection Management**: Store MinIO server URLs, credentials, and TLS configuration
- **Health Monitoring**: Automated health checks with configurable intervals
- **Secret Integration**: Secure credential storage using Kubernetes secrets
- **Cross-Resource References**: Other CRDs can reference aliases for connection details
- **Status Reporting**: Rich status information including server version and health

## Usage

### Basic Alias

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: Alias
metadata:
  name: minio-production
spec:
  url: "https://minio.example.com"
  secretRef:
    name: minio-credentials
    accessKeyIDKey: "accessKeyID"
    secretAccessKeyKey: "secretAccessKey"
  region: "us-east-1"
```

### Alias with Health Checks

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: Alias
metadata:
  name: minio-dev
spec:
  url: "https://dev-minio.example.com"
  secretRef:
    name: dev-minio-credentials
  healthCheck:
    enabled: true
    intervalSeconds: 300
    timeoutSeconds: 30
    failureThreshold: 3
  description: "Development MinIO instance"
  tags:
    environment: "development"
    team: "backend"
```

### Referencing Aliases in Other Resources

Once an alias is defined, other resources can reference it:

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: Bucket
metadata:
  name: my-bucket
spec:
  connection:
    aliasRef:
      name: minio-production
  bucketName: "application-data"
  versioning: true
```

## Migration from Endpoints

The Alias CRD replaces the previous pattern of using `endpointRef` in connections. The old pattern is still supported for backward compatibility:

**Old Pattern (deprecated):**
```yaml
spec:
  connection:
    endpointRef:
      name: endpoint-sample
    secretRef:
      name: minio-credentials
```

**New Pattern (recommended):**
```yaml
spec:
  connection:
    aliasRef:
      name: minio-production
```

## Status Information

The Alias controller provides detailed status information:

```yaml
status:
  ready: true
  healthy: true
  url: "https://minio.example.com"
  version: "RELEASE.2024-01-16T16-07-38Z"
  region: "us-east-1"
  connectedAt: "2024-01-16T10:00:00Z"
  lastHealthCheck: "2024-01-16T10:05:00Z"
  conditions:
  - type: Ready
    status: "True"
    reason: Ready
    message: "Alias is ready"
```

## Best Practices

1. **Use descriptive names**: Choose alias names that clearly identify the MinIO instance (e.g., `minio-production`, `minio-staging`)

2. **Enable health checks**: Configure appropriate health check intervals for production workloads

3. **Secure credentials**: Always use Kubernetes secrets for storing MinIO credentials

4. **Tag your aliases**: Use tags to organize and identify aliases by environment, team, or purpose

5. **Monitor status**: Check alias status regularly to ensure connectivity to MinIO instances

## Troubleshooting

### Alias not ready
- Check that the MinIO server URL is accessible from the cluster
- Verify that credentials in the referenced secret are correct
- Ensure network policies allow communication to the MinIO server

### Health checks failing
- Verify MinIO server is running and responding
- Check TLS configuration if using HTTPS
- Review health check timeout and failure threshold settings

### Connection refused
- Confirm the MinIO server URL and port are correct
- Check firewall rules and network connectivity
- Verify TLS configuration matches the server setup