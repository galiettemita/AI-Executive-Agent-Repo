from __future__ import annotations

import asyncio
from datetime import datetime, timezone
from pathlib import Path

from app.blueprint.mcp.mock_servers import mock_tools_for
from app.blueprint.mcp.normalization import classify_mcp_tool_risk
from app.blueprint.mcp.wave2_catalog import WAVE2_SPECS, build_wave2_manifests, bootstrap_wave2_servers
from app.blueprint.mcp.wave3_catalog import WAVE3_SPECS, build_wave3_manifests, bootstrap_wave3_servers
from app.blueprint.mcp.wave4_catalog import WAVE4_SPECS, build_wave4_manifests, bootstrap_wave4_servers
from app.core.config import settings
from app.db.database import SessionLocal
from app.services.provisioning_catalog import all_catalog_entries

REPORT_PATH = Path("docs/reports/phase4_wave2_4_deployment_checklist.md")


def _manifest_map():
    manifests = [
        *build_wave2_manifests(transport_mode="mock"),
        *build_wave3_manifests(transport_mode="mock"),
        *build_wave4_manifests(transport_mode="mock"),
    ]
    return {m.server_id: m for m in manifests}


def _wave_spec_rows():
    rows = []
    for spec in WAVE2_SPECS:
        rows.append(("wave2", spec.server_id, spec.env_url_name))
    for spec in WAVE3_SPECS:
        rows.append(("wave3", spec.server_id, spec.env_url_name))
    for spec in WAVE4_SPECS:
        rows.append(("wave4", spec.server_id, spec.env_url_name))
    return rows


def _connected_map() -> dict[str, bool]:
    db = SessionLocal()
    try:
        user_id = "phase4-checklist-user"
        wave2 = asyncio.run(bootstrap_wave2_servers(db, user_id=user_id, transport_mode="mock", connect=True))
        wave3 = asyncio.run(bootstrap_wave3_servers(db, user_id=user_id, transport_mode="mock", connect=True))
        wave4 = asyncio.run(bootstrap_wave4_servers(db, user_id=user_id, transport_mode="mock", connect=True))
    finally:
        db.close()

    connected: dict[str, bool] = {}
    for summary in (wave2, wave3, wave4):
        for item in summary.get("items") or []:
            server_id = str(item.get("server_id") or "").strip().lower()
            if not server_id:
                continue
            connected[server_id] = bool(item.get("connected"))
    return connected


def run() -> None:
    manifests = _manifest_map()
    db_seed = SessionLocal()
    try:
        catalog_rows = all_catalog_entries(db_seed)
    finally:
        db_seed.close()
    seed_map = {str(item.get("server_id") or "").strip().lower(): item for item in catalog_rows}
    connected = _connected_map()

    total_checks = 0
    passed_checks = 0
    lines = [
        "# Phase 4 Wave 2-4 Deployment Checklist Report",
        "",
        f"Generated at: {datetime.now(timezone.utc).isoformat()}",
        "",
        "| Wave | Server | Passed | Total |",
        "|---|---|---:|---:|",
    ]

    for wave, server_id, env_name in _wave_spec_rows():
        manifest = manifests.get(server_id)
        tools = mock_tools_for(server_id)
        checks = {
            "manifest_present": manifest is not None,
            "display_name_set": bool(manifest and manifest.display_name.strip()),
            "description_set": bool(manifest and manifest.description.strip()),
            "expected_tools_set": bool(manifest and manifest.expected_tools),
            "tags_set": bool(manifest and manifest.tags),
            "transport_mock_command": bool(manifest and str(manifest.transport.command or "").startswith("mock://")),
            "env_var_declared": hasattr(settings, env_name),
            "mock_tools_registered": len(tools) > 0,
            "catalog_seeded": server_id in seed_map,
            "bootstrap_connected": bool(connected.get(server_id)),
            "tool_schema_probe": all(bool(str(tool.name or "").strip()) for tool in tools),
            "risk_classification": all(classify_mcp_tool_risk(tool.name) is not None for tool in tools),
        }
        passed = sum(1 for ok in checks.values() if ok)
        total = len(checks)
        total_checks += total
        passed_checks += passed
        lines.append(f"| {wave} | `{server_id}` | {passed} | {total} |")

    lines.extend(
        [
            "",
            f"Summary: {passed_checks}/{total_checks} checks passed across Wave 2-4 servers.",
            "",
            "Checklist definition (12 checks per server): manifest, metadata, expected tools, tags, transport, env mapping, mock tools, catalog seeding, bootstrap connectivity, schema probe, risk classification.",
        ]
    )

    REPORT_PATH.parent.mkdir(parents=True, exist_ok=True)
    REPORT_PATH.write_text("\n".join(lines) + "\n", encoding="utf-8")
    print(str(REPORT_PATH))


if __name__ == "__main__":
    run()
