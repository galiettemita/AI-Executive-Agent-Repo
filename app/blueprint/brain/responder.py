from __future__ import annotations

import hashlib
import json
import logging
import random
import time
from typing import Any

import httpx
from sqlalchemy import text

from app.blueprint.contracts import LLMRequest
from app.blueprint.contracts import ContentProvenance, ToolCall, ToolResult
from app.blueprint.context_compiler import compile_context_messages, compile_tool_schemas
from app.blueprint.knowledge_files import get_latest_knowledge_file
from app.blueprint.llm.degraded_retry import enqueue_degraded_retry, should_send_degraded_notice
from app.blueprint.llm.router import LLMDegradedModeQueuedError, get_llm_router
from app.blueprint.temporal_orchestration import orchestrate_tier3_plan
from app.blueprint.tools import get_tool_registry
from app.core.config import settings
from app.db.database import SessionLocal
from app.services.provisioning_catalog import find_server_match, parse_available_servers_section
from app.services.semantic_cache import get_cached_response, put_cached_response

logger = logging.getLogger(__name__)


SYSTEM_PROMPT = (
    "You are Executive OS, a WhatsApp-first executive assistant.\n"
    "Priorities:\n"
    "1) Be concise and action-oriented.\n"
    "2) If required info is missing, ask exactly one clarifying question.\n"
    "3) Never invent user data; if unsure, ask.\n"
    "4) Keep replies under 1200 characters unless the user explicitly asks for detail.\n"
)


def tier0_reply(user_text: str) -> str:
    t = (user_text or "").strip().lower()
    if any(
        t.startswith(x)
        for x in (
            "hi",
            "hello",
            "hey",
            "yo",
            "good morning",
            "good afternoon",
            "good evening",
        )
    ):
        return "Hey — what do you want to get done right now?"
    if t.startswith("thanks") or t.startswith("thank you") or t.startswith("thx"):
        return "Anytime. What’s next?"
    return "Got it. What should I do next?"


def _default_hands_base_url() -> str | None:
    if settings.HANDS_INTERNAL_BASE_URL:
        return settings.HANDS_INTERNAL_BASE_URL.rstrip("/")
    if settings.ENV in ("staging", "production"):
        # Requires ECS Cloud Map namespace `executive-os.local`.
        return "http://hands.executive-os.local:8000"
    return None


def _tool_idempotency_key(*, run_id: str | None, tool: str, args: dict[str, Any]) -> str:
    payload = json.dumps({"run_id": run_id, "tool": tool, "args": args}, sort_keys=True, ensure_ascii=False)
    digest = hashlib.sha256(payload.encode("utf-8")).hexdigest()
    return f"tool:{tool}:{digest}"


def _execute_tool(
    *,
    tool: str,
    args: dict[str, Any],
    user_id: str | None,
    run_id: str | None,
    input_provenance: ContentProvenance,
    required_capabilities: list[str] | None = None,
    capability_token: str | None = None,
    timeout_s: float = 10.0,
) -> ToolResult:
    """
    Execute a tool via Hands internal API (Brain → Hands).
    """
    base = _default_hands_base_url()
    if not base:
        return ToolResult(tool_name=tool, tool=tool, ok=False, error="Hands service not configured")

    call = ToolCall(
        tool_name=tool,
        tool=tool,
        arguments=args or {},
        args=args or {},
        user_id=user_id,
        run_id=run_id,
        input_provenance=input_provenance,
        required_capabilities=required_capabilities or [],
        capability_token=capability_token,
        idempotency_key=_tool_idempotency_key(run_id=run_id, tool=tool, args=args or {}),
    )

    started = time.perf_counter()
    try:
        with httpx.Client(timeout=timeout_s) as client:
            resp = client.post(f"{base}/internal/hands/execute", json=call.model_dump())
            resp.raise_for_status()
            return ToolResult.model_validate(resp.json())
    except Exception as exc:
        elapsed_ms = int((time.perf_counter() - started) * 1000)
        logger.warning("Hands tool execution failed tool=%s latency_ms=%s err=%s", tool, elapsed_ms, exc)
        return ToolResult(tool_name=tool, tool=tool, ok=False, error=str(exc))


def _adaptive_iteration_limit(*, tier: int, user_text: str, tools_count: int) -> int:
    if tier < 2:
        return 1
    text = (user_text or "").strip()
    # Base loop count by tier, then adapt by complexity.
    limit = 3 if tier == 2 else 5
    if len(text) > 220:
        limit += 1
    if any(k in text.lower() for k in (" and ", " then ", "after that", "also", "plus")):
        limit += 1
    if tools_count >= 6:
        limit += 1
    return max(2, min(10, limit))


def _semantic_validate_tool_args(*, tool_schema: dict[str, Any], args: dict[str, Any]) -> str | None:
    required = [str(k) for k in (tool_schema.get("required") or [])]
    for field in required:
        val = args.get(field)
        if val is None:
            return f"Missing required argument: {field}"
        if isinstance(val, str) and not val.strip():
            return f"Argument '{field}' cannot be blank"
    props = tool_schema.get("properties") or {}
    for key, val in args.items():
        spec = props.get(key)
        if not isinstance(spec, dict):
            continue
        typ = str(spec.get("type") or "")
        if typ == "integer" and not isinstance(val, int):
            return f"Argument '{key}' must be integer"
        if typ == "boolean" and not isinstance(val, bool):
            return f"Argument '{key}' must be boolean"
        if typ == "string" and not isinstance(val, str):
            return f"Argument '{key}' must be string"
    return None


def _maybe_reflect_response(
    *,
    llm_router,
    tier: int,
    user_id: str | None,
    user_text: str,
    current_response: str,
) -> tuple[str, dict[str, Any]]:
    if tier < 2:
        return current_response, {"applied": False}

    schema = {
        "type": "object",
        "properties": {
            "needs_revision": {"type": "boolean"},
            "revised_response": {"type": "string"},
            "reason": {"type": "string"},
        },
        "required": ["needs_revision", "revised_response", "reason"],
    }
    reflection_req = LLMRequest(
        user_id=user_id,
        prompt_group="evaluator_prompt_reflection",
        task_type="knowledge_extraction",
        temperature=0.1,
        max_tokens=250,
        structured_output=schema,
        messages=[
            {
                "role": "system",
                "content": (
                    "You are a strict quality reviewer. Return compact JSON only. "
                    "If the response is already correct, set needs_revision=false."
                ),
            },
            {
                "role": "user",
                "content": (
                    f"User request:\n{user_text}\n\n"
                    f"Current response:\n{current_response}\n\n"
                    "Check completeness, safety, and actionability."
                ),
            },
        ],
    )
    try:
        reflection = llm_router.call(reflection_req)
        parsed = json.loads(reflection.content or "{}")
        needs_revision = bool(parsed.get("needs_revision"))
        revised = str(parsed.get("revised_response") or "").strip()
        reason = str(parsed.get("reason") or "").strip()
        if needs_revision and revised:
            return revised, {"applied": True, "reason": reason}
        return current_response, {"applied": False, "reason": reason}
    except Exception as exc:
        logger.warning("self-reflection skipped: %s", exc)
        return current_response, {"applied": False, "error": str(exc)}


def _decompose_subtasks(user_text: str) -> list[str]:
    text = (user_text or "").strip()
    if not text:
        return []
    separators = [" then ", " and ", ";", "\n"]
    parts = [text]
    for sep in separators:
        next_parts: list[str] = []
        for item in parts:
            if sep in item.lower():
                split_items = [s.strip() for s in item.split(sep) if s.strip()]
                next_parts.extend(split_items or [item])
            else:
                next_parts.append(item)
        parts = next_parts
    deduped: list[str] = []
    seen = set()
    for p in parts:
        key = p.lower()
        if key in seen:
            continue
        seen.add(key)
        deduped.append(p)
    return deduped[:8]


def _load_tools_markdown(*, user_id: str | None) -> str:
    if not user_id:
        return ""
    db = SessionLocal()
    try:
        latest = get_latest_knowledge_file(db, user_id=user_id, file_path="TOOLS.md")
        return str((latest or {}).get("content") or "")
    except Exception:
        return ""
    finally:
        try:
            db.close()
        except Exception:
            pass


def _attempt_capability_gap_provision(
    *,
    raw_tool_name: str,
    user_text: str,
    user_id: str | None,
    run_id: str | None,
    provenance: ContentProvenance,
    capability_token: str | None,
) -> ToolResult | None:
    tools_markdown = _load_tools_markdown(user_id=user_id)
    available_servers = parse_available_servers_section(tools_markdown)
    capability_text = " ".join(part for part in [raw_tool_name, user_text] if part).strip()
    match = find_server_match(capability_text, entries=available_servers) if available_servers else None

    if match:
        server_id = str(match.get("server_id") or "").strip()
        if server_id:
            reason = (
                f"Missing tool '{raw_tool_name or 'unknown'}' for request: "
                f"{(user_text or '').strip()[:180]}"
            )
            return _execute_tool(
                tool="provision_server",
                args={"server_id": server_id, "reason": reason},
                user_id=user_id,
                run_id=run_id,
                input_provenance=provenance,
                required_capabilities=[],
                capability_token=capability_token,
                timeout_s=10.0,
            )

    if not capability_text:
        return None
    # Local catalog has no match; ask the remote catalog for candidate servers.
    return _execute_tool(
        tool="search_remote_catalog",
        args={"capability": capability_text},
        user_id=user_id,
        run_id=run_id,
        input_provenance=provenance,
        required_capabilities=[],
        capability_token=capability_token,
        timeout_s=10.0,
    )


def _score_plan(plan: list[str]) -> float:
    score = 0.0
    for idx, item in enumerate(plan):
        weight = max(0.1, 1.0 - (idx * 0.08))
        text = item.lower()
        if any(k in text for k in ("search", "find", "research", "look up")):
            score += 1.2 * weight
        elif any(k in text for k in ("draft", "summarize", "plan")):
            score += 0.9 * weight
        elif any(k in text for k in ("send", "book", "buy", "create")):
            score += 0.7 * weight
        else:
            score += 0.5 * weight
    return score


def _mcts_best_plan(subtasks: list[str], *, num_rollouts: int = 32) -> list[str]:
    if len(subtasks) <= 1:
        return subtasks
    best = list(subtasks)
    best_score = _score_plan(best)
    for _ in range(max(1, num_rollouts)):
        candidate = list(subtasks)
        random.shuffle(candidate)
        score = _score_plan(candidate)
        if score > best_score:
            best = candidate
            best_score = score
    return best


def _load_compensating_actions(*, run_id: str | None, user_id: str | None) -> list[dict[str, Any]]:
    if not run_id or not user_id:
        return []
    db = SessionLocal()
    try:
        rows = db.execute(
            text(
                """
                select compensating_action
                from tool_executions
                where run_id = :run_id
                  and user_id = :user_id
                  and status in ('success', 'completed')
                  and compensating_action is not null
                order by created_at desc
                """
            ),
            {"run_id": run_id, "user_id": user_id},
        ).mappings().all()
    except Exception:
        return []
    finally:
        try:
            db.close()
        except Exception:
            pass

    out: list[dict[str, Any]] = []
    for row in rows:
        value = row.get("compensating_action")
        parsed: dict[str, Any] | None = None
        if isinstance(value, dict):
            parsed = value
        elif isinstance(value, str):
            try:
                candidate = json.loads(value)
                if isinstance(candidate, dict):
                    parsed = candidate
            except Exception:
                parsed = None
        if parsed and parsed.get("tool"):
            out.append(parsed)
    return out


def _run_saga_compensation(
    *,
    run_id: str | None,
    user_id: str | None,
    capability_token: str | None,
) -> dict[str, Any]:
    actions = _load_compensating_actions(run_id=run_id, user_id=user_id)
    if not actions:
        return {"attempted": 0, "succeeded": 0, "failed": 0}

    registry = get_tool_registry()
    attempted = 0
    succeeded = 0
    failed = 0
    details: list[dict[str, Any]] = []
    for action in actions:
        tool_name = str(action.get("tool") or "").strip()
        args = action.get("arguments") or {}
        if not tool_name or not isinstance(args, dict):
            continue
        attempted += 1
        required_capabilities: list[str] = []
        try:
            required_capabilities = list((registry.get(tool_name).capability_scope or []))
        except Exception:
            required_capabilities = []

        result = _execute_tool(
            tool=tool_name,
            args=args,
            user_id=user_id,
            run_id=run_id,
            input_provenance=ContentProvenance.USER_DIRECT,
            required_capabilities=required_capabilities,
            capability_token=capability_token,
            timeout_s=10.0,
        )
        if result.ok:
            succeeded += 1
        else:
            failed += 1
        details.append(
            {
                "tool": tool_name,
                "ok": bool(result.ok),
                "error": result.error if not result.ok else None,
            }
        )
    return {"attempted": attempted, "succeeded": succeeded, "failed": failed, "details": details}


def _prepend_degraded_notice(*, user_id: str | None, body: str) -> tuple[str, bool]:
    text = str(body or "").strip()
    if not should_send_degraded_notice(user_id):
        return (text, False)
    notice = "Heads up: some AI providers are temporarily down, so I’m using backup mode."
    if not text:
        return (notice, True)
    return (f"{notice}\n\n{text}", True)


def generate_reply(
    *,
    user_text: str,
    tier: int,
    user_id: str | None = None,
    conversation_id: str | None = None,
    run_id: str | None = None,
    history_messages: list[dict[str, Any]] | None = None,
    input_provenance: ContentProvenance | str = ContentProvenance.USER_DIRECT,
    capability_token: str | None = None,
    emotion_detected: str | None = None,
    emotion_sensitivity: float | None = None,
) -> tuple[str, dict[str, Any]]:
    """
    Returns (reply_text, meta).
    Meta contains model + token usage when available.
    """
    if tier == 0:
        return tier0_reply(user_text), {"tier": 0, "model": "none"}

    model = settings.OPENAI_MODEL or "gpt-4o-mini"
    provenance = input_provenance
    if isinstance(provenance, str):
        raw = provenance.strip() or ContentProvenance.USER_DIRECT.value
        if raw == "mcp_response":
            raw = ContentProvenance.MCP_RESULT.value
        try:
            provenance = ContentProvenance(raw)
        except Exception:
            provenance = ContentProvenance.USER_DIRECT

    try:
        cache_ctx = {"conversation_id": conversation_id} if conversation_id else None
        if user_id:
            cached = get_cached_response(
                user_id=user_id,
                query_text=user_text,
                model=model,
                tier=tier,
                context=cache_ctx,
            )
            if cached:
                return cached, {"tier": tier, "model": model, "cache_hit": True}

        llm_router = get_llm_router()

        dynamic_system_prompt = SYSTEM_PROMPT
        sensitivity = float(emotion_sensitivity or 0.5)
        emotion = str(emotion_detected or "").strip().lower()
        if sensitivity >= 0.4 and emotion in {"frustrated", "stressed", "rushed"}:
            dynamic_system_prompt += (
                "\nTone adaptation:\n"
                "- Acknowledge urgency/emotion briefly.\n"
                "- Prioritize direct next actions.\n"
                "- Avoid verbose explanations."
            )
        elif sensitivity >= 0.4 and emotion in {"positive", "excited"}:
            dynamic_system_prompt += (
                "\nTone adaptation:\n"
                "- Keep a confident, upbeat tone while staying concise."
            )

        messages, knowledge_files_injected, context_chunks = compile_context_messages(
            base_system_prompt=dynamic_system_prompt,
            user_id=user_id,
            user_text=user_text,
            history_messages=history_messages,
            tier=tier,
        )
        planned_subtasks: list[dict[str, Any]] = []
        tier3_orchestration_meta: dict[str, Any] = {}
        if tier >= 3:
            mcts_ordered = _mcts_best_plan(
                _decompose_subtasks(user_text),
                num_rollouts=24,
            )
            planned_subtasks, tier3_orchestration_meta = orchestrate_tier3_plan(
                run_id=run_id,
                user_id=user_id,
                subtasks=mcts_ordered,
            )
            if planned_subtasks:
                plan_lines = [
                    (
                        f"{idx + 1}. [T{int(step.get('tier') or 1)}]"
                        f" {str(step.get('task') or '')}"
                        f" (retry_limit={int(step.get('retry_limit') or 1)})"
                    )
                    for idx, step in enumerate(planned_subtasks)
                    if str(step.get("task") or "").strip()
                ]
                messages.insert(
                    1,
                    {
                        "role": "system",
                        "content": (
                            "Hierarchical execution plan (Tier 3). "
                            "Complete earlier subtasks before later ones when possible:\n"
                            + "\n".join(plan_lines)
                        ),
                    },
                )
                context_chunks.append(
                    {
                        "role": "system",
                        "source": "tier3_hierarchical_plan",
                        "provenance": "planner",
                    }
                )
        tool_registry = get_tool_registry()
        tools = compile_tool_schemas(tier=tier, user_id=user_id) if tier >= 2 else []
        max_iterations = _adaptive_iteration_limit(tier=tier, user_text=user_text, tools_count=len(tools))

        tool_calls_used = 0
        iterations = 0
        total_input_tokens = 0
        total_output_tokens = 0
        total_latency_ms = 0
        total_cost_cents = 0.0
        provider_name: str | None = None
        prompt_version_id: str | None = None
        msg = ""
        replan_count = 0
        max_replans = 3 if tier >= 3 else 0
        subtask_retry_state: dict[int, int] = {}
        last_failed_tools = 0
        while True:
            iterations += 1
            if tier >= 3:
                task_type = "complex_reasoning"
            elif tier == 2:
                task_type = "single_tool_call" if tools else "general"
            elif tier <= 1:
                task_type = "intent_classification"
            else:
                task_type = "general"

            try:
                resp = llm_router.call(
                    LLMRequest(
                        user_id=user_id,
                        prompt_group="system_prompt",
                        messages=messages,
                        tools=tools or None,
                        temperature=0.4,
                        task_type=task_type,
                        max_tokens=800 if tier <= 1 else 1400,
                    )
                )
            except LLMDegradedModeQueuedError as exc:
                retry_meta = enqueue_degraded_retry(
                    run_id=run_id,
                    user_id=user_id,
                    conversation_id=conversation_id,
                    user_text=user_text,
                    tier=tier,
                    task_type=task_type,
                    reason=str(exc),
                )
                queued_body = "I queued this advanced request and will retry it automatically when full capacity returns."
                queued_text, degraded_notice_sent = _prepend_degraded_notice(user_id=user_id, body=queued_body)
                return (
                    queued_text,
                    {
                        "tier": tier,
                        "model": model,
                        "provider": "local",
                        "degraded_mode": True,
                        "degraded_notice_sent": degraded_notice_sent,
                        "queued_for_retry": bool(retry_meta.get("queued")),
                        "retry_queue": retry_meta,
                    },
                )
            provider_name = resp.provider.value
            model = resp.model
            prompt_version_id = resp.prompt_version_id
            total_input_tokens += resp.usage.input_tokens
            total_output_tokens += resp.usage.output_tokens
            total_latency_ms += resp.latency_ms
            total_cost_cents += resp.cost_cents

            m_content = resp.content
            tool_calls = resp.tool_calls

            if tool_calls and tools and tool_calls_used < max_iterations:
                # Append assistant message with tool_calls
                messages.append(
                    {
                        "role": "assistant",
                        "content": m_content or "",
                        "tool_calls": [
                            {
                                "id": tc.get("id"),
                                "type": "function",
                                "function": {
                                    "name": ((tc.get("function") or {}).get("name")),
                                    "arguments": ((tc.get("function") or {}).get("arguments")),
                                },
                            }
                            for tc in (tool_calls or [])
                        ],
                    }
                )
                context_chunks.append(
                    {
                        "role": "assistant",
                        "source": "llm_tool_plan",
                        "provenance": "assistant_output",
                    }
                )

                # Execute tools and append tool results
                failed_tools = 0
                active_subtask_idx = 0
                for tc in (tool_calls or []):
                    if tool_calls_used >= max_iterations:
                        break
                    tool_calls_used += 1
                    func = tc.get("function") or {}
                    raw_tool_name = str(func.get("name") or "")
                    tool_name = tool_registry.resolve_tool_name(raw_tool_name)
                    tool_args_json = str(func.get("arguments") or "{}")
                    tool_call_id = str(tc.get("id") or f"tool-{tool_calls_used}")
                    try:
                        # Validate tool exists in registry before execution.
                        spec = tool_registry.get(tool_name)
                    except Exception:
                        provision_result = _attempt_capability_gap_provision(
                            raw_tool_name=raw_tool_name,
                            user_text=user_text,
                            user_id=user_id,
                            run_id=run_id,
                            provenance=provenance,
                            capability_token=capability_token,
                        )
                        if provision_result is not None:
                            result = provision_result
                            tool_name = str(
                                provision_result.tool_name
                                or provision_result.tool
                                or "provision_server"
                            )
                            if not result.ok:
                                failed_tools += 1
                        else:
                            result = ToolResult(
                                tool_name=tool_name or raw_tool_name or "unknown",
                                tool=tool_name or raw_tool_name or "unknown",
                                ok=False,
                                error="Unsupported tool",
                            )
                            failed_tools += 1
                    else:
                        try:
                            parsed = json.loads(tool_args_json or "{}")
                        except Exception:
                            parsed = {}
                        parsed_args = parsed if isinstance(parsed, dict) else {}
                        validation_error = _semantic_validate_tool_args(
                            tool_schema=spec.input_schema or {},
                            args=parsed_args,
                        )
                        if validation_error:
                            result = ToolResult(
                                tool_name=tool_name,
                                tool=tool_name,
                                ok=False,
                                error=f"Semantic validation failed: {validation_error}",
                            )
                            failed_tools += 1
                        else:
                            result = _execute_tool(
                                tool=tool_name,
                                args=parsed_args,
                                user_id=user_id,
                                run_id=run_id,
                                input_provenance=provenance,
                                required_capabilities=spec.capability_scope,
                                capability_token=capability_token,
                                timeout_s=10.0,
                            )
                            if not result.ok:
                                failed_tools += 1
                    if planned_subtasks:
                        active_subtask_idx = min(
                            len(planned_subtasks) - 1,
                            max(0, tool_calls_used - 1),
                        )
                    messages.append(
                        {
                            "role": "tool",
                            "tool_call_id": tool_call_id,
                            "content": json.dumps(result.model_dump(), ensure_ascii=False),
                        }
                    )
                    context_chunks.append(
                        {
                            "role": "tool",
                            "source": f"tool:{tool_name}",
                            "provenance": "tool_result",
                        }
                    )
                last_failed_tools = failed_tools
                if tier >= 3 and failed_tools:
                    retry_limit = 1
                    subtask_name = ""
                    if planned_subtasks:
                        step = planned_subtasks[active_subtask_idx]
                        retry_limit = int(step.get("retry_limit") or 1)
                        subtask_name = str(step.get("task") or "").strip()
                    used = subtask_retry_state.get(active_subtask_idx, 0)
                    if used < retry_limit and replan_count < max_replans:
                        subtask_retry_state[active_subtask_idx] = used + 1
                        replan_count += 1
                        checkpoint_text = (
                            f"Checkpoint critic: {failed_tools} tool call(s) failed for "
                            f"subtask '{subtask_name or 'current_step'}'. "
                            "Re-plan this subtask with safer alternatives and reduced side effects."
                        )
                        messages.append({"role": "assistant", "content": checkpoint_text})
                        context_chunks.append(
                            {
                                "role": "assistant",
                                "source": "tier3_checkpoint_critic",
                                "provenance": "planner",
                            }
                        )
                if iterations < max_iterations:
                    continue

            # Final answer
            msg = (m_content or "").strip()
            break

            # Safety stop.
        if iterations >= max_iterations and not msg:
            msg = "I gathered partial results, but hit a planning limit. I can continue if you want."

        saga_compensation_meta: dict[str, Any] = {"attempted": 0, "succeeded": 0, "failed": 0}
        if tier >= 3 and last_failed_tools > 0:
            saga_compensation_meta = _run_saga_compensation(
                run_id=run_id,
                user_id=user_id,
                capability_token=capability_token,
            )
            if int(saga_compensation_meta.get("attempted") or 0) > 0:
                msg = (
                    f"{msg}\n\n"
                    f"Saga compensation executed: "
                    f"{int(saga_compensation_meta.get('succeeded') or 0)} succeeded, "
                    f"{int(saga_compensation_meta.get('failed') or 0)} failed."
                ).strip()

        if tier >= 2:
            msg, reflection_meta = _maybe_reflect_response(
                llm_router=llm_router,
                tier=tier,
                user_id=user_id,
                user_text=user_text,
                current_response=msg or "",
            )
        else:
            reflection_meta = {"applied": False}

        degraded_mode = False
        degraded_notice_sent = False
        try:
            degraded_mode = llm_router.system_mode(force_refresh=False) == "degraded"
        except Exception:
            degraded_mode = False
        if degraded_mode:
            msg, degraded_notice_sent = _prepend_degraded_notice(user_id=user_id, body=msg or "")

        meta: dict[str, Any] = {"tier": tier, "model": model}
        if provider_name:
            meta["provider"] = provider_name
        if prompt_version_id:
            meta["prompt_version_id"] = prompt_version_id
        if degraded_mode:
            meta["degraded_mode"] = True
            meta["degraded_notice_sent"] = degraded_notice_sent
        meta["knowledge_files_injected"] = knowledge_files_injected
        meta["context_chunks"] = context_chunks
        if planned_subtasks:
            meta["planned_subtasks"] = planned_subtasks
            meta["mcts_rollouts"] = 24
            meta["orchestration"] = tier3_orchestration_meta
            meta["subtask_retry_state"] = subtask_retry_state
        if int(saga_compensation_meta.get("attempted") or 0) > 0:
            meta["saga_compensation"] = saga_compensation_meta
        meta["iterations"] = iterations
        meta["max_iterations"] = max_iterations
        meta["self_reflection"] = reflection_meta
        meta["input_provenance"] = provenance.value
        if emotion:
            meta["emotion_detected"] = emotion
            meta["emotion_sensitivity"] = round(sensitivity, 3)
        if capability_token:
            meta["capability_token_applied"] = True
        meta["usage"] = {
            "input_tokens": total_input_tokens,
            "output_tokens": total_output_tokens,
            "total_tokens": total_input_tokens + total_output_tokens,
        }
        meta["latency_ms"] = total_latency_ms
        meta["cost_cents"] = round(total_cost_cents, 4)

        if user_id and msg:
            try:
                put_cached_response(
                    user_id=user_id,
                    query_text=user_text,
                    assistant_message=msg,
                    model=model,
                    tier=tier,
                    context=cache_ctx,
                )
            except Exception:
                pass

        return msg or "I’m not sure I got that — can you rephrase?", meta
    except Exception:
        logger.exception("LLM reply generation failed")
        return (
            "I hit an internal error generating a reply. Try again in a minute.",
            {"tier": tier, "model": model, "error": True},
        )
