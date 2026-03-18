#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="brevio"
ISTIO_VERSION="1.21.0"

echo "Installing Istio ${ISTIO_VERSION}..."

# Add Istio Helm repo.
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update

# Install istio-base (CRDs).
helm upgrade --install istio-base istio/base \
  --namespace istio-system --create-namespace \
  -f infra/helm/istio-base-values.yaml \
  --version "${ISTIO_VERSION}"

# Install istiod.
helm upgrade --install istiod istio/istiod \
  --namespace istio-system \
  -f infra/helm/istiod-values.yaml \
  --version "${ISTIO_VERSION}" \
  --wait

# Install istio-ingress.
helm upgrade --install istio-ingress istio/gateway \
  --namespace istio-ingress --create-namespace \
  -f infra/helm/istio-ingress-values.yaml \
  --version "${ISTIO_VERSION}"

# Enable sidecar injection on brevio namespace.
kubectl label namespace "${NAMESPACE}" istio-injection=enabled --overwrite

echo "Istio installation complete."
