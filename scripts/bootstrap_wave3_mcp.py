from __future__ import annotations

import argparse
import asyncio
import json

from app.blueprint.mcp.wave3_catalog import bootstrap_wave3_servers
from app.db.database import SessionLocal


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Bootstrap Wave 3 MCP server manifests")
    parser.add_argument("--user-id", required=True, help="Owner user id for mcp_user_servers binding")
    parser.add_argument(
        "--transport-mode",
        default=None,
        help="Transport mode override: mock | streamable_http | stdio",
    )
    parser.add_argument(
        "--no-connect",
        action="store_true",
        help="Register manifests only; skip connect/probe",
    )
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    db = SessionLocal()
    try:
        summary = asyncio.run(
            bootstrap_wave3_servers(
                db,
                user_id=args.user_id,
                transport_mode=args.transport_mode,
                connect=not args.no_connect,
            )
        )
    finally:
        db.close()

    print(json.dumps(summary, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main()
