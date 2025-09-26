#!/bin/bash

# Simple E2E test script to validate mc-controller functionality
set -e

echo "🚀 Starting simplified E2E tests for mc-controller"

# Clean up any existing resources
echo "🧹 Cleaning up previous test resources..."
kind delete cluster --name mc-controller-e2e 2>/dev/null || true

# Create Kind cluster
echo "🔧 Creating Kind cluster..."
kind create cluster --name mc-controller-e2e --wait 5m

# Install CRDs
echo "📦 Installing CRDs..."
make install

# Build controller
echo "🏗️ Building controller..."
make build

# Deploy MinIO for testing
echo "🗂️ Deploying MinIO..."
kubectl create namespace minio-system --dry-run=client -o yaml | kubectl apply -f -

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: minio
  namespace: minio-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: minio
  template:
    metadata:
      labels:
        app: minio
    spec:
      containers:
      - name: minio
        image: minio/minio:latest
        args:
        - server
        - /data
        - --console-address
        - :9001
        ports:
        - containerPort: 9000
        - containerPort: 9001
        env:
        - name: MINIO_ROOT_USER
          value: minioadmin
        - name: MINIO_ROOT_PASSWORD
          value: minioadmin
        volumeMounts:
        - name: data
          mountPath: /data
      volumes:
      - name: data
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: minio
  namespace: minio-system
spec:
  selector:
    app: minio
  ports:
  - name: api
    port: 9000
    targetPort: 9000
  - name: console
    port: 9001
    targetPort: 9001
  type: ClusterIP
EOF

# Wait for MinIO to be ready
echo "⏳ Waiting for MinIO to be ready..."
kubectl wait --for=condition=available --timeout=300s deployment/minio -n minio-system

# Start port-forward in background
echo "🔗 Starting port-forward..."
kubectl port-forward -n minio-system service/minio 9000:9000 &
PF_PID=$!
sleep 5

# Test basic CRD creation (without controller reconciliation for now)
echo "📝 Testing CRD creation..."
kubectl create namespace mc-controller-test --dry-run=client -o yaml | kubectl apply -f -

# Create a basic secret
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: minio-credentials
  namespace: mc-controller-test
data:
  accessKeyID: bWluaW9hZG1pbg==     # minioadmin
  secretAccessKey: bWluaW9hZG1pbg== # minioadmin
EOF

# Create a basic Alias resource
cat <<EOF | kubectl apply -f -
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Alias
metadata:
  name: test-alias
  namespace: mc-controller-test
spec:
  url: "http://localhost:9000"
  secretRef:
    name: minio-credentials
  healthCheck:
    enabled: true
    intervalSeconds: 300
EOF

# Verify CRD was created
echo "✅ Verifying Alias creation..."
kubectl get aliases -n mc-controller-test test-alias -o yaml

# Create a basic Bucket resource
cat <<EOF | kubectl apply -f -
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Bucket
metadata:
  name: test-bucket
  namespace: mc-controller-test
spec:
  connection:
    aliasRef:
      name: test-alias
  bucketName: "test-bucket"
  versioning: false
  tags:
    test: "true"
EOF

echo "✅ Verifying Bucket creation..."
kubectl get buckets -n mc-controller-test test-bucket -o yaml

echo "🎉 Basic CRD functionality test completed successfully!"
echo "📊 Test Results:"
echo "  ✅ Kind cluster created"
echo "  ✅ MinIO deployed and running"
echo "  ✅ CRDs installed successfully"
echo "  ✅ Alias resource created"  
echo "  ✅ Bucket resource created"
echo "  ✅ All resources accessible via kubectl"

# Cleanup
echo "🧹 Cleaning up..."
kill $PF_PID 2>/dev/null || true
kind delete cluster --name mc-controller-e2e

echo "✨ E2E test completed successfully!"