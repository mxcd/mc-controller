# mc-controller

A comprehensive Kubernetes operator for managing MinIO resources including buckets, users, policies, lifecycle policies, policy attachments, and endpoints.

## Overview

mc-controller is a Kubernetes operator built with Kubebuilder that provides declarative management of MinIO resources through Custom Resource Definitions (CRDs). It enables teams to manage MinIO infrastructure as code using familiar Kubernetes primitives.

## Features

- **Alias Management**: Centralized MinIO connection configuration similar to `mc alias` 
- **Bucket Management**: Create and manage MinIO buckets with versioning, object locking, notifications, and quotas
- **User Management**: Manage MinIO users with password rotation and group memberships
- **Policy Management**: Define and attach IAM policies for access control
- **Lifecycle Policies**: Configure automatic object expiration and storage class transitions
- **Endpoint Management**: Manage MinIO server connections with health checks (deprecated, use Alias)
- **Policy Attachments**: Attach policies to users, groups, or service accounts
- **Idempotent Operations**: Safely reconcile desired state with actual MinIO configuration
- **Finalizers**: Proper cleanup of resources when deleted from Kubernetes

## Custom Resource Definitions

### Alias

Centralized MinIO connection configuration (recommended approach):

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: Alias
metadata:
  name: minio-production
spec:
  url: "https://minio.example.com"
  secretRef:
    name: minio-admin-credentials
    accessKeyIDKey: "accessKeyID"
    secretAccessKeyKey: "secretAccessKey"
  tls:
    insecure: false
  healthCheck:
    enabled: true
    intervalSeconds: 300
  region: "us-east-1"
  description: "Production MinIO instance"
```

### Endpoint

Defines connection details to a MinIO server (deprecated, use Alias):

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: Endpoint
metadata:
  name: minio-production
spec:
  url: "https://minio.example.com"
  secretRef:
    name: minio-admin-credentials
    accessKeyIDKey: "accessKeyID"
    secretAccessKeyKey: "secretAccessKey"
  tls:
    insecure: false
  healthCheck:
    enabled: true
    intervalSeconds: 300
  region: "us-east-1"
```

### Bucket

Creates and manages MinIO buckets:

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: Bucket
metadata:
  name: data-bucket
spec:
  connection:
    aliasRef:
      name: minio-production
  bucketName: "application-data"
  versioning: true
  objectLocking: false
  tags:
    team: "backend"
    environment: "production"
```

### User

Manages MinIO users:

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: User
metadata:
  name: app-user
spec:
  connection:
    endpointRef:
      name: minio-production
    secretRef:
      name: minio-admin-credentials
  username: "application-user"
  secretRef:
    name: user-password
    secretAccessKeyKey: "password"
  status: "enabled"
  policies:
    - "readwrite"
```

### Policy

Defines IAM policies:

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: Policy
metadata:
  name: readonly-policy
spec:
  connection:
    endpointRef:
      name: minio-production
    secretRef:
      name: minio-admin-credentials
  policyName: "readonly-access"
  policy: |
    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Effect": "Allow",
          "Action": [
            "s3:GetObject",
            "s3:ListBucket"
          ],
          "Resource": [
            "arn:aws:s3:::application-data/*",
            "arn:aws:s3:::application-data"
          ]
        }
      ]
    }
```

### PolicyAttachment

Attaches policies to users or groups:

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: PolicyAttachment
metadata:
  name: readonly-for-viewer
spec:
  connection:
    endpointRef:
      name: minio-production
    secretRef:
      name: minio-admin-credentials
  policyName: "readonly-access"
  target:
    user: "viewer-user"
```

### LifecyclePolicy

Configures bucket lifecycle management:

```yaml
apiVersion: minio.mxcd.dev/v1alpha1
kind: LifecyclePolicy
metadata:
  name: data-retention
spec:
  connection:
    endpointRef:
      name: minio-production
    secretRef:
      name: minio-admin-credentials
  bucketName: "application-data"
  rules:
    - id: "delete-old-objects"
      status: "Enabled"
      expiration:
        days: 365
      filter:
        prefix: "logs/"
```

## Installation

### Prerequisites

- Kubernetes cluster (v1.21+)
- kubectl configured to access your cluster
- MinIO server accessible from the cluster

### Install the Operator

1. Install the CRDs:
   ```bash
   kubectl apply -f config/crd/bases/
   ```

2. Create the namespace and RBAC:
   ```bash
   kubectl apply -f config/rbac/
   ```

3. Deploy the operator:
   ```bash
   kubectl apply -f config/manager/
   ```

### Quick Start

1. Create a secret with MinIO credentials:
   ```bash
   kubectl apply -f config/samples/minio-credentials-secret.yaml
   ```

2. Create an endpoint:
   ```bash
   kubectl apply -f config/samples/minio_v1alpha1_endpoint.yaml
   ```

3. Create a bucket:
   ```bash
   kubectl apply -f config/samples/minio_v1alpha1_bucket.yaml
   ```

## Development

### Prerequisites

- Go 1.21+
- Kubebuilder 3.14+
- Docker (for building images)

### Building

```bash
# Generate code and manifests
make generate manifests

# Build the manager binary
make build

# Run tests
make test

# Build Docker image
make docker-build
```

### Running Locally

```bash
# Install CRDs
make install

# Run the controller locally
make run
```

## Architecture

The mc-controller follows the standard Kubernetes operator pattern:

- **Controllers**: Implement reconciliation logic for each CRD
- **Finalizers**: Ensure proper cleanup when resources are deleted
- **Status Conditions**: Provide visibility into resource state
- **MinIO Clients**: Wrapped minio-go v7 (S3) and madmin-go v3 (admin) clients
- **Connection Management**: Centralized handling of MinIO connections with credential management

## Security

- Credentials are stored in Kubernetes secrets
- TLS/SSL support with certificate validation
- RBAC permissions follow the principle of least privilege
- Finalizers prevent accidental data loss

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.