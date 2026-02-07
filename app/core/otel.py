from __future__ import annotations

import logging

from app.core.config import settings

logger = logging.getLogger(__name__)


def setup_otel(app) -> None:
    if not settings.OTEL_ENABLED:
        return

    if not settings.OTEL_EXPORTER_OTLP_ENDPOINT:
        logger.warning("OTEL enabled but OTEL_EXPORTER_OTLP_ENDPOINT is not set")
        return

    try:
        from opentelemetry import trace
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
        from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
        from opentelemetry.instrumentation.requests import RequestsInstrumentor
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor

        resource = Resource.create({"service.name": settings.OTEL_SERVICE_NAME})
        provider = TracerProvider(resource=resource)

        headers = None
        if settings.OTEL_EXPORTER_OTLP_HEADERS:
            headers = dict(
                item.split("=", 1) for item in settings.OTEL_EXPORTER_OTLP_HEADERS.split(",") if "=" in item
            )

        exporter = OTLPSpanExporter(
            endpoint=settings.OTEL_EXPORTER_OTLP_ENDPOINT,
            headers=headers,
        )
        processor = BatchSpanProcessor(exporter)
        provider.add_span_processor(processor)
        trace.set_tracer_provider(provider)

        FastAPIInstrumentor.instrument_app(app)
        RequestsInstrumentor().instrument()
        logger.info("OpenTelemetry tracing enabled")
    except Exception as exc:
        logger.error("Failed to initialize OpenTelemetry: %s", exc)
