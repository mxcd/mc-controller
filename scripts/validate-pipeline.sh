#!/bin/bash
set -e

echo "🚀 Validating mc-controller pipeline..."

echo "📦 Installing dependencies..."
go mod download

echo "🧪 Running unit tests..."
make test

echo "🔍 Running linter..."
make lint

echo "📄 Validating manifests..."
make generate manifests
if ! git diff --exit-code; then
    echo "❌ Generated files are not up to date. Please run 'make generate manifests' and commit the changes."
    exit 1
fi

echo "📋 Validating Helm chart..."
if command -v helm &> /dev/null; then
    helm lint charts/mc-controller
    echo "✅ Helm chart validation passed"
else
    echo "⚠️  Helm not installed, skipping chart validation"
fi

echo "🔒 Running security scan..."
if command -v gosec &> /dev/null; then
    gosec ./...
    echo "✅ Security scan passed"
else
    echo "⚠️  gosec not installed, skipping security scan"
fi

echo "✅ All pipeline validations passed!"