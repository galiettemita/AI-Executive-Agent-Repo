#!/usr/bin/env bash
set -euo pipefail

echo "Installing Kiali..."

helm repo add kiali https://kiali.org/helm-charts
helm repo update

helm upgrade --install kiali-server kiali/kiali-server \
  --namespace istio-system \
  -f infra/k8s/istio/kiali-values.yaml

echo "Kiali installation complete."
