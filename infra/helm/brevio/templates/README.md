# Brevio Umbrella Chart Templates

This chart composes existing core service subcharts and renders additional core services
that do not yet have standalone chart directories in `helm/`.

Rendered in this template set:
- `ServiceAccount` for all rendered in-chart services
- `Deployment` + `Service` + optional `HorizontalPodAutoscaler` for
  `additionalServices` entries in values

Subchart-backed services are configured via chart dependencies in `Chart.yaml`.
