from __future__ import annotations

import importlib.util
from pathlib import Path


def _load_migration_module():
    path = Path("alembic/versions/w7x8y9z0a1b2_phase1_v5_foundation_schema.py")
    spec = importlib.util.spec_from_file_location("baseline_v5_migration", path)
    assert spec is not None and spec.loader is not None
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_baseline_migration_contains_operational_identity_and_billing_tables() -> None:
    module = _load_migration_module()
    sql = module._postgres_only_sql().lower()

    users_pos = sql.find("create table if not exists users")
    accounts_pos = sql.find("create table if not exists accounts")
    channels_pos = sql.find("create table if not exists channel_connections")
    subscriptions_pos = sql.find("create table if not exists subscriptions")
    invoices_pos = sql.find("create table if not exists invoices")
    runs_pos = sql.find("create table if not exists runs")

    assert users_pos != -1
    assert accounts_pos != -1
    assert channels_pos != -1
    assert subscriptions_pos != -1
    assert invoices_pos != -1
    assert runs_pos != -1

    # Enforce baseline order required by the integration directive.
    assert users_pos < accounts_pos < channels_pos < subscriptions_pos < invoices_pos < runs_pos
