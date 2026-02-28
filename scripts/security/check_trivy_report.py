#!/usr/bin/env python3

import json
import pathlib
import sys


def load_allowlist(path: pathlib.Path) -> set[str]:
    allowlist: set[str] = set()
    for line in path.read_text(encoding="utf-8").splitlines():
        token = line.strip()
        if not token or token.startswith("#"):
            continue
        allowlist.add(token)
    return allowlist


def parse_findings(report: dict) -> list[dict]:
    findings: list[dict] = []
    for result in report.get("Results", []) or []:
        target = result.get("Target", "")
        for vuln in result.get("Vulnerabilities", []) or []:
            severity = str(vuln.get("Severity", "")).upper()
            if severity not in {"HIGH", "CRITICAL"}:
                continue
            findings.append(
                {
                    "id": str(vuln.get("VulnerabilityID", "")).strip(),
                    "severity": severity,
                    "package": str(vuln.get("PkgName", "")),
                    "installed": str(vuln.get("InstalledVersion", "")),
                    "fixed": str(vuln.get("FixedVersion", "")),
                    "target": target,
                }
            )
    return findings


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: check_trivy_report.py <trivy_report.json> <allowlist.txt>", file=sys.stderr)
        return 2

    report_path = pathlib.Path(sys.argv[1])
    allowlist_path = pathlib.Path(sys.argv[2])
    if not report_path.exists() or report_path.stat().st_size == 0:
        print(f"[trivy-check] report missing or empty: {report_path}", file=sys.stderr)
        return 1
    if not allowlist_path.exists() or allowlist_path.stat().st_size == 0:
        print(f"[trivy-check] allowlist missing or empty: {allowlist_path}", file=sys.stderr)
        return 1

    report = json.loads(report_path.read_text(encoding="utf-8"))
    allowlist = load_allowlist(allowlist_path)
    findings = parse_findings(report)
    if not findings:
        print("[trivy-check] no HIGH/CRITICAL vulnerabilities found")
        return 0

    blocking = [item for item in findings if item["id"] not in allowlist]
    if blocking:
        print("[trivy-check] blocking HIGH/CRITICAL vulnerabilities detected:")
        for item in blocking:
            print(
                f"  - {item['id']} ({item['severity']}) "
                f"pkg={item['package']} installed={item['installed']} fixed={item['fixed']} target={item['target']}"
            )
        return 1

    unique_allowlisted = sorted({item["id"] for item in findings})
    print("[trivy-check] only allowlisted HIGH/CRITICAL vulnerabilities found:", ", ".join(unique_allowlisted))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
