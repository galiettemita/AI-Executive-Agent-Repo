from __future__ import annotations

import logging

from app.core.config import settings

logger = logging.getLogger(__name__)


def _parse_headers(raw: str | None) -> dict[str, str] | None:
    if not raw:
        return None
    out: dict[str, str] = {}
    for item in raw.split(","):
        if "=" not in item:
            continue
        key, value = item.split("=", 1)
        key = key.strip()
        value = value.strip()
        if key:
            out[key] = value
    return out or None


def setup_otel(app) -> None:
    if not settings.OTEL_ENABLED:
        return

    traces_endpoint = (
        settings.OTEL_EXPORTER_OTLP_TRACES_ENDPOINT
        or settings.OTEL_EXPORTER_OTLP_ENDPOINT
    )
    if not traces_endpoint:
        logger.warning("OTEL enabled but OTEL_EXPORTER_OTLP_ENDPOINT is not set")
        return

    try:
        from opentelemetry import metrics, trace
        from opentelemetry.exporter.otlp.proto.http.metric_exporter import OTLPMetricExporter
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
        from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
        from opentelemetry.instrumentation.requests import RequestsInstrumentor
        from opentelemetry.sdk.metrics import MeterProvider
        from opentelemetry.sdk.metrics.export import PeriodicExportingMetricReader
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor

        resource = Resource.create({"service.name": settings.OTEL_SERVICE_NAME})
        provider = TracerProvider(resource=resource)

        trace_headers = (
            _parse_headers(settings.OTEL_EXPORTER_OTLP_TRACES_HEADERS)
            or _parse_headers(settings.OTEL_EXPORTER_OTLP_HEADERS)
        )

        exporter = OTLPSpanExporter(
            endpoint=traces_endpoint,
            headers=trace_headers,
        )
        processor = BatchSpanProcessor(exporter)
        provider.add_span_processor(processor)
        trace.set_tracer_provider(provider)

        # Prefer an explicit metrics endpoint. If unset and traces endpoint points at /v1/traces,
        # skip metrics export to avoid shipping metrics payloads to a traces-only endpoint.
        metrics_endpoint = settings.OTEL_EXPORTER_OTLP_METRICS_ENDPOINT
        if not metrics_endpoint and settings.OTEL_EXPORTER_OTLP_ENDPOINT:
            base_endpoint = settings.OTEL_EXPORTER_OTLP_ENDPOINT
            metrics_endpoint = (
                None if base_endpoint.rstrip("/").endswith("/v1/traces") else base_endpoint
            )

        metric_readers = []
        if settings.OTEL_METRICS_ENABLED and metrics_endpoint:
            metric_headers = (
                _parse_headers(settings.OTEL_EXPORTER_OTLP_METRICS_HEADERS)
                or _parse_headers(settings.OTEL_EXPORTER_OTLP_HEADERS)
            )
            metric_exporter = OTLPMetricExporter(
                endpoint=metrics_endpoint,
                headers=metric_headers,
            )
            metric_readers.append(PeriodicExportingMetricReader(metric_exporter))
        else:
            logger.info("OpenTelemetry metrics exporter disabled")

        meter_provider = MeterProvider(resource=resource, metric_readers=metric_readers)
        metrics.set_meter_provider(meter_provider)

        # Instrumentation (fallback for older versions that don't accept meter_provider)
        try:
            FastAPIInstrumentor.instrument_app(app, tracer_provider=provider, meter_provider=meter_provider)
        except TypeError:
            FastAPIInstrumentor.instrument_app(app, tracer_provider=provider)
        try:
            RequestsInstrumentor().instrument(meter_provider=meter_provider)
        except TypeError:
            RequestsInstrumentor().instrument()
        logger.info("OpenTelemetry tracing enabled")
    except Exception as exc:
        logger.error("Failed to initialize OpenTelemetry: %s", exc)
