#!/usr/bin/env bash
set -euo pipefail

ROOT_DOMAIN="${ROOT_DOMAIN:-}"
ACM_CERT_ARN="${ACM_CERT_ARN:-}"
API_HOST="${API_HOST:-}"
ADMIN_HOST="${ADMIN_HOST:-}"
OUTPUT_DIR="${OUTPUT_DIR:-artifacts/deploy}"

if [[ -z "${ROOT_DOMAIN}" ]]; then
  echo "ROOT_DOMAIN is required (example: testing-orbit.com)" >&2
  exit 1
fi

if [[ -z "${ACM_CERT_ARN}" ]]; then
  echo "ACM_CERT_ARN is required" >&2
  exit 1
fi

if [[ -z "${API_HOST}" ]]; then
  API_HOST="api.${ROOT_DOMAIN}"
fi

if [[ -z "${ADMIN_HOST}" ]]; then
  ADMIN_HOST="admin.${ROOT_DOMAIN}"
fi

mkdir -p "${OUTPUT_DIR}"

GATEWAY_VALUES_FILE="${OUTPUT_DIR}/gateway-prod-values.yaml"
ADMIN_FRONTEND_VALUES_FILE="${OUTPUT_DIR}/admin-frontend-prod-values.yaml"

cat >"${GATEWAY_VALUES_FILE}" <<EOF
service:
  type: ClusterIP
ingress:
  enabled: true
  className: alb
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/certificate-arn: ${ACM_CERT_ARN}
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443}]'
    alb.ingress.kubernetes.io/ssl-redirect: '443'
    alb.ingress.kubernetes.io/healthcheck-path: /healthz/live
    external-dns.alpha.kubernetes.io/hostname: ${API_HOST}
  hosts:
    - host: ${API_HOST}
      paths:
        - path: /
          pathType: Prefix
EOF

cat >"${ADMIN_FRONTEND_VALUES_FILE}" <<EOF
service:
  type: ClusterIP
ingress:
  enabled: true
  className: alb
  annotations:
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/target-type: ip
    alb.ingress.kubernetes.io/certificate-arn: ${ACM_CERT_ARN}
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443}]'
    alb.ingress.kubernetes.io/ssl-redirect: '443'
    alb.ingress.kubernetes.io/healthcheck-path: /healthz/live
    external-dns.alpha.kubernetes.io/hostname: ${ADMIN_HOST}
  hosts:
    - host: ${ADMIN_HOST}
      paths:
        - path: /
          pathType: Prefix
EOF

cat <<EOF
Generated:
  ${GATEWAY_VALUES_FILE}
  ${ADMIN_FRONTEND_VALUES_FILE}

Next commands:
  helm upgrade --install brevio-gateway helm/BREVIO-gateway -f ${GATEWAY_VALUES_FILE}
  helm upgrade --install brevio-admin-frontend helm/BREVIO-admin-frontend -f ${ADMIN_FRONTEND_VALUES_FILE}
EOF
