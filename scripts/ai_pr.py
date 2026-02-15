#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


MAX_FILES_REQUESTED = 25
MAX_FILE_CHARS = 24_000
MAX_TOTAL_CONTEXT_CHARS = 220_000
PATCH_PATH = Path("/tmp/ai_pr.patch")


def _die(msg: str) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(2)


def _run(cmd: list[str], *, check: bool = True) -> subprocess.CompletedProcess[str]:
    # Avoid logging secrets; commands are safe to print.
    print("+ " + " ".join(cmd))
    return subprocess.run(cmd, check=check, text=True, capture_output=True)


def _git_output(args: list[str]) -> str:
    p = _run(["git", *args])
    return (p.stdout or "").strip()


def _slug(s: str) -> str:
    s = (s or "").strip().lower()
    s = re.sub(r"[^a-z0-9]+", "-", s)
    s = re.sub(r"-{2,}", "-", s).strip("-")
    return s[:32] or "task"


def _read_text(path: str) -> str:
    p = Path(path)
    data = p.read_text(encoding="utf-8", errors="replace")
    if len(data) > MAX_FILE_CHARS:
        data = data[:MAX_FILE_CHARS] + "\n\n/* TRUNCATED */\n"
    return data


def _collect_repo_files() -> list[str]:
    out = _git_output(["ls-files"])
    files = [line.strip() for line in out.splitlines() if line.strip()]
    return files


def _build_files_context(files: list[str]) -> str:
    total = 0
    blocks: list[str] = []
    for f in files:
        if not f or f.startswith(".git/"):
            continue
        if Path(f).suffix.lower() in {".png", ".jpg", ".jpeg", ".gif", ".pdf", ".zip"}:
            continue
        try:
            content = _read_text(f)
        except Exception as e:
            blocks.append(f'<file path="{f}">/* unreadable: {e} */</file>')
            continue
        block = f'<file path="{f}">\n{content}\n</file>'
        if total + len(block) > MAX_TOTAL_CONTEXT_CHARS:
            blocks.append("/* CONTEXT TRUNCATED: reached max context size */")
            break
        blocks.append(block)
        total += len(block)
    return "\n\n".join(blocks)


@dataclass
class FileRequest:
    needed_files: list[str]
    proposed_tests: list[str]
    pr_title: str


@dataclass
class PatchRequest:
    diff: str
    commit_message: str
    pr_title: str
    pr_body: str
    tests: list[str]


def _openai_client(api_key: str):
    try:
        from openai import OpenAI  # type: ignore
    except Exception as e:  # pragma: no cover
        _die(f"openai python package not available: {e}")
    return OpenAI(api_key=api_key)


def _chat_json(client: Any, *, model: str, messages: list[dict[str, str]]) -> dict[str, Any]:
    resp = client.chat.completions.create(
        model=model,
        messages=messages,
        response_format={"type": "json_object"},
        temperature=0.2,
    )
    content = resp.choices[0].message.content or "{}"
    try:
        return json.loads(content)
    except Exception as e:
        _die(f"Model returned invalid JSON: {e}\nRaw:\n{content}")
    raise AssertionError("unreachable")


def _select_files(client: Any, *, model: str, prompt: str, repo_files: list[str]) -> FileRequest:
    system = (
        "You are an autonomous senior software engineer working in a GitHub Actions runner.\n"
        "You must respond with STRICT JSON only (no markdown). \n"
        "Goal: choose the minimum set of files needed to implement the request.\n"
        "Constraints:\n"
        f"- Return at most {MAX_FILES_REQUESTED} file paths in needed_files.\n"
        "- Only choose paths that exist in the provided repo file list.\n"
        "- Prefer app/, infra/, scripts/, tests/, docs/.\n"
    )
    user = (
        f"REQUEST:\n{prompt}\n\n"
        "REPO_FILES (git ls-files):\n"
        + "\n".join(repo_files)
        + "\n\n"
        "Return JSON with keys:\n"
        "- needed_files: string[]\n"
        "- proposed_tests: string[] (shell commands)\n"
        "- pr_title: string\n"
    )
    data = _chat_json(
        client,
        model=model,
        messages=[{"role": "system", "content": system}, {"role": "user", "content": user}],
    )
    needed_files = [str(x) for x in (data.get("needed_files") or []) if str(x)]
    proposed_tests = [str(x) for x in (data.get("proposed_tests") or []) if str(x)]
    pr_title = str(data.get("pr_title") or "AI change")
    # Enforce validity
    repo_set = set(repo_files)
    needed_files = [f for f in needed_files if f in repo_set][:MAX_FILES_REQUESTED]
    return FileRequest(needed_files=needed_files, proposed_tests=proposed_tests, pr_title=pr_title)


def _request_patch(
    client: Any,
    *,
    model: str,
    prompt: str,
    repo_files: list[str],
    needed_files: list[str],
    files_context: str,
    prior_error: str | None = None,
) -> PatchRequest:
    system = (
        "You are an autonomous senior software engineer.\n"
        "You MUST respond with STRICT JSON only (no markdown).\n"
        "You are given a repository file list and the content of selected files.\n"
        "Produce a unified diff that applies cleanly with `git apply`.\n"
        "Rules:\n"
        "- Only modify files in the repository.\n"
        "- Avoid touching secrets and .env.\n"
        "- Keep diffs minimal.\n"
        "- If you need additional files, expand needed_files within the response.\n"
    )
    user_parts = [
        f"REQUEST:\n{prompt}\n",
        "REPO_FILES (git ls-files):\n" + "\n".join(repo_files),
        "SELECTED_FILES:\n" + "\n".join(needed_files),
        "SELECTED_FILE_CONTENTS:\n" + files_context,
    ]
    if prior_error:
        user_parts.append("PATCH/APPLY/TEST FAILURE CONTEXT:\n" + prior_error)
    user_parts.append(
        "Return JSON with keys:\n"
        "- diff: string (unified diff for git apply)\n"
        "- commit_message: string\n"
        "- pr_title: string\n"
        "- pr_body: string\n"
        "- tests: string[] (shell commands)\n"
    )
    data = _chat_json(
        client,
        model=model,
        messages=[{"role": "system", "content": system}, {"role": "user", "content": "\n\n".join(user_parts)}],
    )
    diff = str(data.get("diff") or "")
    commit_message = str(data.get("commit_message") or "AI change")
    pr_title = str(data.get("pr_title") or "AI change")
    pr_body = str(data.get("pr_body") or "")
    tests = [str(x) for x in (data.get("tests") or []) if str(x)]
    if not diff.strip():
        _die("Model returned empty diff.")
    return PatchRequest(diff=diff, commit_message=commit_message, pr_title=pr_title, pr_body=pr_body, tests=tests)


def _apply_patch(diff: str) -> str | None:
    PATCH_PATH.write_text(diff, encoding="utf-8")
    p = _run(["git", "apply", "--whitespace=nowarn", str(PATCH_PATH)], check=False)
    if p.returncode == 0:
        return None
    return (p.stderr or p.stdout or "").strip() or f"git apply failed (code {p.returncode})"


def _run_tests(cmds: list[str]) -> tuple[bool, str]:
    if not cmds:
        cmds = ["PYTHONPATH=. pytest -q"]
    outputs: list[str] = []
    for cmd in cmds:
        p = subprocess.run(cmd, shell=True, text=True, capture_output=True)
        outputs.append(f"$ {cmd}\n{p.stdout}\n{p.stderr}".strip())
        if p.returncode != 0:
            return False, "\n\n".join(outputs)
    return True, "\n\n".join(outputs)


def _ensure_dirty() -> None:
    if not _git_output(["status", "--porcelain"]):
        _die("No changes produced by patch; refusing to open empty PR.")


def main() -> None:
    prompt = os.getenv("CODEGEN_PROMPT", "").strip()
    if not prompt:
        _die("CODEGEN_PROMPT is required.")

    api_key = (os.getenv("CODEGEN_OPENAI_API_KEY") or os.getenv("OPENAI_API_KEY") or "").strip()
    if not api_key:
        _die("Missing CODEGEN_OPENAI_API_KEY (recommended) or OPENAI_API_KEY.")

    model = (os.getenv("CODEGEN_MODEL") or "gpt-4o-mini").strip()
    base_branch = (os.getenv("CODEGEN_BASE_BRANCH") or "main").strip()
    run_tests_flag = (os.getenv("CODEGEN_RUN_TESTS") or "true").lower() in {"1", "true", "yes", "on"}

    client = _openai_client(api_key)
    repo_files = _collect_repo_files()

    # Create a fresh branch
    ts = datetime.now(timezone.utc).strftime("%Y%m%d-%H%M%S")
    branch = f"codex/ai-{ts}-{_slug(prompt)}"
    _run(["git", "checkout", base_branch])
    _run(["git", "pull", "--ff-only", "origin", base_branch])
    _run(["git", "checkout", "-b", branch])

    # 1) Ask which files are needed
    fr = _select_files(client, model=model, prompt=prompt, repo_files=repo_files)
    needed_files = fr.needed_files or []
    # Fallback: include a small default set if the model returns nothing.
    if not needed_files:
        needed_files = [p for p in ["CHECKLIST.md", "app/main.py", "app/core/config.py"] if p in set(repo_files)]

    files_context = _build_files_context(needed_files)

    # 2) Request a patch and try applying/testing up to 2 times
    prior_error: str | None = None
    patch: PatchRequest | None = None
    for attempt in range(1, 3):
        patch = _request_patch(
            client,
            model=model,
            prompt=prompt,
            repo_files=repo_files,
            needed_files=needed_files,
            files_context=files_context,
            prior_error=prior_error,
        )
        apply_err = _apply_patch(patch.diff)
        if apply_err:
            prior_error = f"Attempt {attempt}: git apply failed.\n{apply_err}"
            continue
        if run_tests_flag:
            ok, out = _run_tests(patch.tests)
            if not ok:
                prior_error = f"Attempt {attempt}: tests failed.\n{out}"
                continue
        prior_error = None
        break

    if prior_error:
        _die(prior_error)
    assert patch is not None

    _ensure_dirty()

    _run(["git", "add", "-A"])
    _run(["git", "commit", "-m", patch.commit_message])
    _run(["git", "push", "-u", "origin", branch])

    pr_title = patch.pr_title.strip() or fr.pr_title.strip() or "AI change"
    pr_body = patch.pr_body.strip() or f"Automated change for:\n\n{prompt}"

    # Create PR (gh uses GH_TOKEN)
    p = _run(
        [
            "gh",
            "pr",
            "create",
            "--base",
            base_branch,
            "--head",
            branch,
            "--title",
            pr_title,
            "--body",
            pr_body,
        ]
    )
    url = (p.stdout or "").strip()
    print(f"PR: {url}")


if __name__ == "__main__":
    main()

