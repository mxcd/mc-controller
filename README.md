# mc-controller

A comprehensive Kubernetes operator for managing MinIO resources including buckets, users, policies, lifecycle policies, policy attachments, and aliases.

## Overview

mc-controller is a Kubernetes operator built with Kubebuilder that provides declarative management of MinIO resources through Custom Resource Definitions (CRDs). It enables teams to manage MinIO infrastructure as code using familiar Kubernetes primitives, similar to the `mc` CLI tool but with the power of Kubernetes automation.

## Features

- **🔗 Alias Management**: Centralized MinIO connection configuration similar to `mc alias`
- **🪣 Bucket Management**: Create and manage MinIO buckets with versioning, object locking, notifications, and quotas
- **👤 User Management**: Manage MinIO users with password rotation and group memberships
- **📋 Policy Management**: Define and attach IAM policies for access control
- **🔄 Lifecycle Policies**: Configure automatic object expiration and storage class transitions
- **🔗 Policy Attachments**: Attach policies to users, groups, or service accounts
- **🔄 Idempotent Operations**: Safely reconcile desired state with actual MinIO configuration
- **🛡️ Finalizers**: Proper cleanup of resources when deleted from Kubernetes
- **📊 Status Reporting**: Rich status information and health monitoring
- **🚀 CI/CD Ready**: Automated testing and deployment pipelines

## Quick Start

### 1. Install the Operator

```bash
# Install CRDs
kubectl apply -f https://github.com/mxcd/mc-controller/releases/latest/download/crds.yaml

# Install the operator
kubectl apply -f https://github.com/mxcd/mc-controller/releases/latest/download/operator.yaml
```

### 2. Create MinIO Credentials Secret

```bash
kubectl create secret generic minio-credentials \
  --from-literal=accessKeyID=admin \
  --from-literal=secretAccessKey=password123
```

### 3. Define a MinIO Alias

```yaml
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Alias
metadata:
  name: minio-local
spec:
  url: "https://minio.local:9000"
  secretRef:
    name: minio-credentials
  healthCheck:
    enabled: true
    intervalSeconds: 300
  region: "us-east-1"
  description: "Local MinIO instance"
```

### 4. Create a Bucket

```yaml
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Bucket
metadata:
  name: my-app-data
spec:
  connection:
    aliasRef:
      name: minio-local
  bucketName: "application-data"
  versioning: true
  tags:
    app: "my-application"
    environment: "production"
```

## Custom Resource Definitions

### Alias

Centralized MinIO connection configuration (recommended approach):

```yaml
apiVersion: mc-controller.mxcd.de/v1alpha1
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

### Bucket

Creates and manages MinIO buckets:

```yaml
apiVersion: mc-controller.mxcd.de/v1alpha1
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
  quota:
    hard: 10737418240  # 10GB
  notification:
    events: ["s3:ObjectCreated:*", "s3:ObjectRemoved:*"]
    topic: "arn:aws:sns:us-east-1:123456789012:my-topic"
```

### User

Manages MinIO users:

```yaml
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: User
metadata:
  name: app-user
spec:
  connection:
    aliasRef:
      name: minio-production
  username: "application-user"
  secretRef:
    name: user-password
    secretAccessKeyKey: "password"
  status: "enabled"
  groups:
    - "developers"
  policies:
    - "readwrite"
```

### Policy

Defines IAM policies:

```yaml
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Policy
metadata:
  name: readonly-policy
spec:
  connection:
    aliasRef:
      name: minio-production
  policyName: "readonly-access"
  description: "Read-only access to application data"
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
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: PolicyAttachment
metadata:
  name: readonly-for-viewer
spec:
  connection:
    aliasRef:
      name: minio-production
  policyName: "readonly-access"
  target:
    user: "viewer-user"
```

### LifecyclePolicy

Configures bucket lifecycle management:

```yaml
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: LifecyclePolicy
metadata:
  name: data-retention
spec:
  connection:
    aliasRef:
      name: minio-production
  bucketName: "application-data"
  rules:
    - id: "delete-old-logs"
      status: "Enabled"
      filter:
        prefix: "logs/"
      expiration:
        days: 30
    - id: "archive-old-data"
      status: "Enabled"
      filter:
        prefix: "archive/"
      transitions:
        - days: 90
          storageClass: "GLACIER"
```

## Common Usage Patterns

### Multi-Environment Setup

```yaml
# Development Environment
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Alias
metadata:
  name: minio-dev
spec:
  url: "https://dev-minio.example.com"
  secretRef:
    name: dev-minio-credentials
  description: "Development MinIO instance"
  tags:
    environment: "development"

---
# Production Environment  
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Alias
metadata:
  name: minio-prod
spec:
  url: "https://prod-minio.example.com"
  secretRef:
    name: prod-minio-credentials
  healthCheck:
    enabled: true
    intervalSeconds: 180
    failureThreshold: 3
  description: "Production MinIO instance"
  tags:
    environment: "production"
```

### Application Data Management

```yaml
# Application bucket
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Bucket
metadata:
  name: myapp-data
spec:
  connection:
    aliasRef:
      name: minio-prod
  bucketName: "myapp-data"
  versioning: true
  tags:
    app: "myapp"
    team: "backend"

---
# Application user
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: User
metadata:
  name: myapp-user
spec:
  connection:
    aliasRef:
      name: minio-prod
  username: "myapp-service"
  secretRef:
    name: myapp-minio-credentials
  status: "enabled"

---
# Application policy
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Policy
metadata:
  name: myapp-policy
spec:
  connection:
    aliasRef:
      name: minio-prod
  policyName: "myapp-access"
  policy: |
    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Effect": "Allow",
          "Action": ["s3:*"],
          "Resource": [
            "arn:aws:s3:::myapp-data/*",
            "arn:aws:s3:::myapp-data"
          ]
        }
      ]
    }

---
# Attach policy to user
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: PolicyAttachment
metadata:
  name: myapp-access
spec:
  connection:
    aliasRef:
      name: minio-prod
  policyName: "myapp-access"
  target:
    user: "myapp-service"
```

### Data Lifecycle Management

```yaml
# Backup bucket with lifecycle
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Bucket
metadata:
  name: backup-bucket
spec:
  connection:
    aliasRef:
      name: minio-prod
  bucketName: "backups"
  versioning: true
  objectLocking: true

---
# Lifecycle policy for backups
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: LifecyclePolicy
metadata:
  name: backup-lifecycle
spec:
  connection:
    aliasRef:
      name: minio-prod
  bucketName: "backups"
  rules:
    - id: "daily-backups"
      status: "Enabled"
      filter:
        prefix: "daily/"
      expiration:
        days: 7
    - id: "weekly-backups"
      status: "Enabled"
      filter:
        prefix: "weekly/"
      expiration:
        days: 30
    - id: "monthly-backups"
      status: "Enabled"
      filter:
        prefix: "monthly/"
      expiration:
        days: 365
```

## Installation

### Prerequisites

- Kubernetes cluster (v1.21+)
- kubectl configured to access your cluster
- MinIO server accessible from the cluster

### Install from Release

```bash
# Install CRDs and operator
kubectl apply -f https://github.com/mxcd/mc-controller/releases/latest/download/install.yaml
```

### Install from Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/mxcd/mc-controller.git
   cd mc-controller
   ```

2. **Install CRDs:**
   ```bash
   make install
   ```

3. **Deploy the operator:**
   ```bash
   make deploy IMG=ghcr.io/mxcd/mc-controller:latest
   ```

### Development Setup

```bash
# Install dependencies
make deps

# Generate code and manifests
make generate manifests

# Run tests
make test

# Run locally (for development)
make run
```

## Monitoring and Observability

### Check Resource Status

```bash
# Check all MinIO resources
kubectl get alias,bucket,user,policy,policyattachment,lifecyclepolicy

# Detailed status for specific resource
kubectl describe bucket my-bucket

# Check operator logs
kubectl logs -n mc-controller-system deployment/mc-controller-controller-manager
```

### Status Conditions

All resources provide comprehensive status information:

```yaml
status:
  ready: true
  conditions:
  - type: Ready
    status: "True"
    lastTransitionTime: "2024-01-16T10:00:00Z"
    reason: Ready
    message: "Resource is ready"
  - type: Progressing
    status: "False"
    reason: Ready
    message: "Resource reconciliation completed"
```

## Architecture

The mc-controller follows the standard Kubernetes operator pattern:

- **Controllers**: Implement reconciliation logic for each CRD
- **Finalizers**: Ensure proper cleanup when resources are deleted  
- **Status Conditions**: Provide visibility into resource state
- **MinIO Clients**: Wrapped minio-go v7 (S3) and madmin-go v3 (admin) clients
- **Connection Management**: Centralized handling of MinIO connections via Aliases

## Security

- **Credentials**: Stored in Kubernetes secrets with configurable key names
- **TLS/SSL**: Full support with certificate validation options
- **RBAC**: Follows principle of least privilege
- **Finalizers**: Prevent accidental data loss during resource deletion

## Troubleshooting

### Common Issues

#### Alias Not Ready

```bash
kubectl describe alias minio-prod
```

**Possible causes:**
- MinIO server not accessible
- Invalid credentials in secret
- Network connectivity issues
- TLS certificate problems

#### Bucket Creation Failed

**Check:**
1. Alias is ready and healthy
2. User has sufficient permissions
3. Bucket name follows MinIO naming conventions
4. No conflicting bucket names

#### Policy Attachment Failed

**Verify:**
1. Policy exists in MinIO
2. Target user/group exists
3. Policy document is valid JSON
4. Admin credentials have sufficient permissions

### Debug Commands

```bash
# Check controller logs
kubectl logs -n mc-controller-system -l control-plane=controller-manager

# Check resource events
kubectl get events --sort-by=.metadata.creationTimestamp

# Validate CRD definitions
kubectl explain alias.spec
kubectl explain bucket.status

# Test MinIO connectivity manually
kubectl run -i --tty debug --image=minio/mc --rm -- /bin/sh
```

### Getting Help

- **Documentation**: Check the [docs/](./docs/) directory
- **Issues**: Report bugs on [GitHub Issues](https://github.com/mxcd/mc-controller/issues)
- **Discussions**: Ask questions in [GitHub Discussions](https://github.com/mxcd/mc-controller/discussions)

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run tests: `make test`
5. Commit your changes: `git commit -m 'Add amazing feature'`
6. Push to the branch: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Development Workflow

```bash
# Setup development environment
make dev-setup

# Run tests
make test

# Run linting
make lint

# Build and test locally
make build
make run

# Build container image
make docker-build IMG=my-registry/mc-controller:dev
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

---

## API Reference

For detailed API documentation, see:

- [Alias CRD](./docs/alias.md)
- [Generated API Docs](./docs/api.md)
- [Examples](./config/samples/)

## Roadmap

- [ ] Webhook validation for CRDs
- [ ] Backup and restore operations
- [ ] Multi-tenant support
- [ ] Advanced monitoring and metrics
- [ ] Object replication management
- [ ] Integration with external secret management systems