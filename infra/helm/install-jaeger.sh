#!/usr/bin/env bash
set -euo pipefail

echo "Installing Jaeger Operator..."

# For production: use Jaeger Operator with Elasticsearch backend.
kubectl create namespace monitoring --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -n monitoring \
  -f https://raw.githubusercontent.com/jaegertracing/jaeger-operator/main/deploy/crds/jaegertracing.io_jaegers.yaml || true

kubectl apply -n monitoring \
  -f https://github.com/jaegertracing/jaeger-operator/releases/latest/download/jaeger-operator.yaml || true

echo "Jaeger Operator installation complete."
