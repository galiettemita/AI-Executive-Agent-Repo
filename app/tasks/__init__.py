# Task package for Celery autodiscovery
#
# Celery autodiscovery can be fragile depending on how it's configured.
# We import task modules explicitly to ensure registration.

from app.tasks import system  # noqa: F401
from app.tasks import inbound_whatsapp  # noqa: F401
