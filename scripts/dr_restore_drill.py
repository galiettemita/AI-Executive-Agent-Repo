#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import sys
from dataclasses import dataclass
from datetime import datetime, timezone

import boto3
from botocore.exceptions import ClientError


@dataclass(frozen=True)
class DrillConfig:
    region: str
    source_db_instance_id: str
    restore_db_subnet_group: str
    restore_vpc_security_group_ids: list[str]
    restore_db_instance_class: str
    restore_id_prefix: str
    wait_timeout_seconds: int
    execute: bool
    cleanup: bool


def _utc_stamp() -> str:
    return datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S")


def _latest_automated_snapshot(rds, source_db_instance_id: str) -> dict:
    paginator = rds.get_paginator("describe_db_snapshots")
    latest: dict | None = None
    for page in paginator.paginate(DBInstanceIdentifier=source_db_instance_id, SnapshotType="automated"):
        for snapshot in page.get("DBSnapshots", []):
            status = str(snapshot.get("Status") or "")
            if status != "available":
                continue
            if latest is None:
                latest = snapshot
                continue
            existing_time = latest.get("SnapshotCreateTime")
            candidate_time = snapshot.get("SnapshotCreateTime")
            if candidate_time and existing_time and candidate_time > existing_time:
                latest = snapshot
    if latest is None:
        raise RuntimeError(f"No available automated snapshots found for DB instance {source_db_instance_id}")
    return latest


def _wait_for_instance_available(rds, *, db_instance_id: str, timeout_seconds: int) -> None:
    waiter = rds.get_waiter("db_instance_available")
    delay = 30
    max_attempts = max(1, int(timeout_seconds / delay))
    waiter.wait(DBInstanceIdentifier=db_instance_id, WaiterConfig={"Delay": delay, "MaxAttempts": max_attempts})


def _delete_temp_instance(rds, *, db_instance_id: str) -> None:
    try:
        rds.delete_db_instance(
            DBInstanceIdentifier=db_instance_id,
            SkipFinalSnapshot=True,
            DeleteAutomatedBackups=True,
        )
    except ClientError as exc:
        code = str(exc.response.get("Error", {}).get("Code") or "")
        if code in {"DBInstanceNotFound", "InvalidDBInstanceState"}:
            return
        raise


def run(cfg: DrillConfig) -> dict:
    rds = boto3.client("rds", region_name=cfg.region)

    snapshot = _latest_automated_snapshot(rds, cfg.source_db_instance_id)
    snapshot_id = str(snapshot.get("DBSnapshotIdentifier") or "")
    restore_id = f"{cfg.restore_id_prefix}-{_utc_stamp()}"[:63]

    result: dict[str, object] = {
        "ok": True,
        "execute": cfg.execute,
        "source_db_instance_id": cfg.source_db_instance_id,
        "snapshot_id": snapshot_id,
        "restore_db_instance_id": restore_id,
        "region": cfg.region,
    }

    if not cfg.execute:
        result["note"] = "Dry run only. No restore API calls executed."
        return result

    try:
        rds.restore_db_instance_from_db_snapshot(
            DBInstanceIdentifier=restore_id,
            DBSnapshotIdentifier=snapshot_id,
            DBInstanceClass=cfg.restore_db_instance_class,
            DBSubnetGroupName=cfg.restore_db_subnet_group,
            VpcSecurityGroupIds=cfg.restore_vpc_security_group_ids,
            PubliclyAccessible=False,
            MultiAZ=False,
            AutoMinorVersionUpgrade=False,
            CopyTagsToSnapshot=True,
        )
        result["restore_started"] = True

        _wait_for_instance_available(
            rds,
            db_instance_id=restore_id,
            timeout_seconds=cfg.wait_timeout_seconds,
        )
        result["restore_available"] = True

        desc = rds.describe_db_instances(DBInstanceIdentifier=restore_id)
        instances = desc.get("DBInstances", [])
        endpoint = None
        if instances:
            endpoint = (instances[0].get("Endpoint") or {}).get("Address")
        result["endpoint"] = endpoint

        if cfg.cleanup:
            _delete_temp_instance(rds, db_instance_id=restore_id)
            result["cleanup_started"] = True

        return result
    except Exception as exc:
        result["ok"] = False
        result["error"] = str(exc)
        raise


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run an RDS restore drill for disaster recovery validation.")
    parser.add_argument("--region", required=True, help="AWS region (example: us-east-1)")
    parser.add_argument("--source-db-instance-id", required=True, help="Production/staging source DB instance ID")
    parser.add_argument("--restore-db-subnet-group", required=True, help="DB subnet group for temporary restore instance")
    parser.add_argument(
        "--restore-vpc-security-group-ids",
        required=True,
        help="Comma-separated security group IDs for restore instance",
    )
    parser.add_argument(
        "--restore-db-instance-class",
        default="db.t4g.medium",
        help="Temporary restore DB class (default: db.t4g.medium)",
    )
    parser.add_argument(
        "--restore-id-prefix",
        default="drill-restore",
        help="Prefix for temporary restore DB instance identifier",
    )
    parser.add_argument(
        "--wait-timeout-seconds",
        type=int,
        default=3600,
        help="Wait timeout for restored instance to become available",
    )
    parser.add_argument(
        "--execute",
        action="store_true",
        help="Execute restore. Without this flag, script runs in dry-run mode.",
    )
    parser.add_argument(
        "--cleanup",
        action="store_true",
        help="Delete temporary restored instance after verification.",
    )
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    sg_ids = [x.strip() for x in str(args.restore_vpc_security_group_ids).split(",") if x.strip()]
    if not sg_ids:
        raise ValueError("At least one security group ID is required")

    cfg = DrillConfig(
        region=args.region,
        source_db_instance_id=args.source_db_instance_id,
        restore_db_subnet_group=args.restore_db_subnet_group,
        restore_vpc_security_group_ids=sg_ids,
        restore_db_instance_class=args.restore_db_instance_class,
        restore_id_prefix=args.restore_id_prefix,
        wait_timeout_seconds=int(args.wait_timeout_seconds),
        execute=bool(args.execute),
        cleanup=bool(args.cleanup),
    )

    result = run(cfg)
    print(json.dumps(result, indent=2, default=str))
    return 0 if bool(result.get("ok")) else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
