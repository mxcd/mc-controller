#!/bin/bash

set -e

echo "Setting up minimal test environment..."

# Create a kind cluster
kind create cluster --name debug-test --wait 2m || true

# Install CRDs
make install

# Build controller
make build

# Start controller in background
./bin/manager &
CONTROLLER_PID=$!

sleep 10

# Install MinIO
kubectl create namespace minio-system --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f - <<EOF
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
        image: minio/minio:RELEASE.2024-01-16T16-07-38Z
        command: ["minio", "server", "/data", "--console-address", ":9001"]
        env:
        - name: MINIO_ROOT_USER
          value: minioadmin
        - name: MINIO_ROOT_PASSWORD
          value: minioadmin
        ports:
        - containerPort: 9000
        - containerPort: 9001
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

# Wait for MinIO
echo "Waiting for MinIO to be ready..."
kubectl wait --for=condition=ready pod -l app=minio -n minio-system --timeout=60s

# Port forward
kubectl port-forward -n minio-system service/minio 9000:9000 &
PF_PID=$!
sleep 5

# Create test namespace and secret
kubectl create namespace test-ns --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic minio-credentials \
  --from-literal=accessKeyID=minioadmin \
  --from-literal=secretAccessKey=minioadmin \
  -n test-ns

kubectl create secret generic test-user-password \
  --from-literal=password=testpassword123 \
  -n test-ns

# Create alias
kubectl apply -f - <<EOF
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: Alias
metadata:
  name: test-alias
  namespace: test-ns
spec:
  url: "http://localhost:9000"
  secretRef:
    name: minio-credentials
EOF

echo "Waiting for alias to be ready..."
timeout=60
while [ $timeout -gt 0 ]; do
  if kubectl get alias test-alias -n test-ns -o jsonpath='{.status.ready}' | grep -q true; then
    echo "Alias is ready"
    break
  fi
  sleep 1
  timeout=$((timeout-1))
done

if [ $timeout -eq 0 ]; then
  echo "Alias failed to become ready"
  kubectl describe alias test-alias -n test-ns
  exit 1
fi

# Create user
kubectl apply -f - <<EOF
apiVersion: mc-controller.mxcd.de/v1alpha1
kind: User
metadata:
  name: test-user-crd
  namespace: test-ns
spec:
  connection:
    aliasRef:
      name: test-alias
  username: test-user
  secretRef:
    name: test-user-password
    secretAccessKeyKey: password
  status: enabled
EOF

echo "Waiting for user to be ready..."
timeout=60
while [ $timeout -gt 0 ]; do
  if kubectl get user test-user-crd -n test-ns -o jsonpath='{.status.ready}' | grep -q true; then
    echo "User is ready"
    break
  fi
  sleep 1
  timeout=$((timeout-1))
done

if [ $timeout -eq 0 ]; then
  echo "User failed to become ready"
  kubectl describe user test-user-crd -n test-ns
  echo ""
  echo "User status:"
  kubectl get user test-user-crd -n test-ns -o yaml
  echo ""
  echo "Recent events:"
  kubectl get events -n test-ns --sort-by='.lastTimestamp'
fi

# Cleanup
echo "Cleaning up..."
kill $CONTROLLER_PID 2>/dev/null || true
kill $PF_PID 2>/dev/null || true
kind delete cluster --name debug-test