from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime
from typing import Any

from sqlalchemy import text
from sqlalchemy.orm import Session

from app.blueprint.mcp.contracts import MCPServerConfig, MCPServerManifest, MCPServerSummary, MCPToolSchema


@dataclass
class RegistryServer:
    config: MCPServerConfig
    tools: list[MCPToolSchema]
    resources: list[dict[str, Any]]
    prompts: list[dict[str, Any]]


def _loads(value: Any, default: Any) -> Any:
    if isinstance(value, (dict, list)):
        return value
    if isinstance(value, str):
        try:
            return json.loads(value)
        except Exception:
            return default
    return default


class MCPServerRegistry:
    def ensure_tables(self, db: Session) -> None:
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    create table if not exists mcp_servers (
                      id integer primary key autoincrement,
                      server_id text unique,
                      display_name text,
                      description text,
                      transport_json text,
                      tags_json text,
                      expected_tools_json text,
                      expected_resources_json text,
                      expected_prompts_json text,
                      tools_json text,
                      resources_json text,
                      prompts_json text,
                      state text default 'registered',
                      rate_limit_per_min integer default 60,
                      daily_budget_cents integer default 500,
                      version text default '1.0.0',
                      health_status text default 'unknown',
                      consecutive_failures integer default 0,
                      total_calls integer default 0,
                      total_errors integer default 0,
                      total_cost_cents real default 0,
                      last_health_check_at text,
                      updated_at text,
                      created_at text
                    )
                    """
                )
            )
            db.execute(
                text(
                    """
                    create table if not exists mcp_user_servers (
                      id integer primary key autoincrement,
                      user_id text,
                      server_id text,
                      is_enabled integer default 1,
                      custom_config_json text,
                      daily_budget_override real,
                      created_at text,
                      unique(user_id, server_id)
                    )
                    """
                )
            )
            db.commit()
            return

        db.execute(
            text(
                """
                create table if not exists mcp_servers (
                  id uuid primary key default gen_random_uuid(),
                  server_id text unique not null,
                  display_name text not null,
                  description text,
                  transport jsonb not null,
                  tags text[] default '{}',
                  expected_tools text[] default '{}',
                  expected_resources text[] default '{}',
                  expected_prompts text[] default '{}',
                  tools jsonb default '[]'::jsonb,
                  resources jsonb default '[]'::jsonb,
                  prompts jsonb default '[]'::jsonb,
                  state text default 'registered',
                  rate_limit_per_min int default 60,
                  daily_budget_cents int default 500,
                  version text default '1.0.0',
                  health_status text default 'unknown',
                  last_health_check_at timestamptz,
                  consecutive_failures int default 0,
                  total_calls int default 0,
                  total_errors int default 0,
                  total_cost_cents float default 0,
                  created_at timestamptz default now(),
                  updated_at timestamptz default now()
                )
                """
            )
        )
        db.execute(
            text(
                """
                create table if not exists mcp_user_servers (
                  id uuid primary key default gen_random_uuid(),
                  user_id uuid not null,
                  server_id text not null references mcp_servers(server_id),
                  is_enabled boolean default true,
                  custom_config jsonb default '{}'::jsonb,
                  daily_budget_override float,
                  created_at timestamptz default now(),
                  unique(user_id, server_id)
                )
                """
            )
        )
        db.execute(text("create index if not exists idx_mcp_servers_state on mcp_servers(state)"))
        db.execute(text("create index if not exists idx_mcp_user_servers_user on mcp_user_servers(user_id)"))
        db.commit()

    def upsert_server(self, db: Session, manifest: MCPServerManifest) -> MCPServerConfig:
        self.ensure_tables(db)
        now = datetime.utcnow().isoformat()
        dialect = db.bind.dialect.name if db.bind is not None else ""

        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    insert into mcp_servers (
                      server_id, display_name, description, transport_json, tags_json,
                      expected_tools_json, expected_resources_json, expected_prompts_json,
                      state, rate_limit_per_min, daily_budget_cents, updated_at, created_at
                    ) values (
                      :server_id, :display_name, :description, :transport_json, :tags_json,
                      :expected_tools_json, :expected_resources_json, :expected_prompts_json,
                      'registered', :rate_limit_per_min, :daily_budget_cents, :now, :now
                    )
                    on conflict(server_id) do update set
                      display_name=excluded.display_name,
                      description=excluded.description,
                      transport_json=excluded.transport_json,
                      tags_json=excluded.tags_json,
                      expected_tools_json=excluded.expected_tools_json,
                      expected_resources_json=excluded.expected_resources_json,
                      expected_prompts_json=excluded.expected_prompts_json,
                      rate_limit_per_min=excluded.rate_limit_per_min,
                      daily_budget_cents=excluded.daily_budget_cents,
                      updated_at=excluded.updated_at
                    """
                ),
                {
                    "server_id": manifest.server_id,
                    "display_name": manifest.display_name,
                    "description": manifest.description,
                    "transport_json": json.dumps(manifest.transport.model_dump(), ensure_ascii=False),
                    "tags_json": json.dumps(manifest.tags, ensure_ascii=False),
                    "expected_tools_json": json.dumps(manifest.expected_tools, ensure_ascii=False),
                    "expected_resources_json": json.dumps(manifest.expected_resources, ensure_ascii=False),
                    "expected_prompts_json": json.dumps(manifest.expected_prompts, ensure_ascii=False),
                    "rate_limit_per_min": manifest.rate_limit_per_min,
                    "daily_budget_cents": manifest.daily_budget_cents,
                    "now": now,
                },
            )
            db.commit()
            return self.get_server_config(db, manifest.server_id)

        db.execute(
            text(
                """
                insert into mcp_servers (
                  server_id, display_name, description, transport, tags,
                  expected_tools, expected_resources, expected_prompts,
                  state, rate_limit_per_min, daily_budget_cents, updated_at
                ) values (
                  :server_id, :display_name, :description, (:transport)::jsonb, :tags,
                  :expected_tools, :expected_resources, :expected_prompts,
                  'registered', :rate_limit_per_min, :daily_budget_cents, now()
                )
                on conflict(server_id) do update set
                  display_name=excluded.display_name,
                  description=excluded.description,
                  transport=excluded.transport,
                  tags=excluded.tags,
                  expected_tools=excluded.expected_tools,
                  expected_resources=excluded.expected_resources,
                  expected_prompts=excluded.expected_prompts,
                  rate_limit_per_min=excluded.rate_limit_per_min,
                  daily_budget_cents=excluded.daily_budget_cents,
                  updated_at=now()
                """
            ),
            {
                "server_id": manifest.server_id,
                "display_name": manifest.display_name,
                "description": manifest.description,
                "transport": json.dumps(manifest.transport.model_dump(), ensure_ascii=False),
                "tags": manifest.tags,
                "expected_tools": manifest.expected_tools,
                "expected_resources": manifest.expected_resources,
                "expected_prompts": manifest.expected_prompts,
                "rate_limit_per_min": manifest.rate_limit_per_min,
                "daily_budget_cents": manifest.daily_budget_cents,
            },
        )
        db.commit()
        return self.get_server_config(db, manifest.server_id)

    def bind_user_server(self, db: Session, *, user_id: str, server_id: str) -> None:
        self.ensure_tables(db)
        dialect = db.bind.dialect.name if db.bind is not None else ""
        now = datetime.utcnow().isoformat()
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    insert into mcp_user_servers (user_id, server_id, is_enabled, custom_config_json, created_at)
                    values (:user_id, :server_id, 1, '{}', :now)
                    on conflict(user_id, server_id) do update set is_enabled=1
                    """
                ),
                {"user_id": user_id, "server_id": server_id, "now": now},
            )
        else:
            db.execute(
                text(
                    """
                    insert into mcp_user_servers (user_id, server_id, is_enabled, custom_config, created_at)
                    values ((:user_id)::uuid, :server_id, true, '{}'::jsonb, now())
                    on conflict(user_id, server_id) do update set is_enabled=true
                    """
                ),
                {"user_id": user_id, "server_id": server_id},
            )
        db.commit()

    def set_server_capabilities(
        self,
        db: Session,
        *,
        server_id: str,
        tools: list[MCPToolSchema],
        resources: list[dict[str, Any]],
        prompts: list[dict[str, Any]],
        state: str = "approved",
        health_status: str = "healthy",
    ) -> None:
        self.ensure_tables(db)
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            db.execute(
                text(
                    """
                    update mcp_servers
                    set tools_json=:tools_json,
                        resources_json=:resources_json,
                        prompts_json=:prompts_json,
                        state=:state,
                        health_status=:health_status,
                        last_health_check_at=:now,
                        updated_at=:now
                    where server_id=:server_id
                    """
                ),
                {
                    "server_id": server_id,
                    "tools_json": json.dumps([t.model_dump() for t in tools], ensure_ascii=False),
                    "resources_json": json.dumps(resources, ensure_ascii=False),
                    "prompts_json": json.dumps(prompts, ensure_ascii=False),
                    "state": state,
                    "health_status": health_status,
                    "now": datetime.utcnow().isoformat(),
                },
            )
        else:
            db.execute(
                text(
                    """
                    update mcp_servers
                    set tools=(:tools)::jsonb,
                        resources=(:resources)::jsonb,
                        prompts=(:prompts)::jsonb,
                        state=:state,
                        health_status=:health_status,
                        last_health_check_at=now(),
                        updated_at=now()
                    where server_id=:server_id
                    """
                ),
                {
                    "server_id": server_id,
                    "tools": json.dumps([t.model_dump() for t in tools], ensure_ascii=False),
                    "resources": json.dumps(resources, ensure_ascii=False),
                    "prompts": json.dumps(prompts, ensure_ascii=False),
                    "state": state,
                    "health_status": health_status,
                },
            )
        db.commit()

    def get_server_config(self, db: Session, server_id: str) -> MCPServerConfig:
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            row = db.execute(
                text(
                    """
                    select server_id, display_name, description,
                           transport_json,
                           tags_json,
                           expected_tools_json,
                           expected_resources_json,
                           expected_prompts_json,
                           state, rate_limit_per_min, daily_budget_cents
                    from mcp_servers where server_id=:server_id
                    """
                ),
                {"server_id": server_id},
            ).mappings().first()
        else:
            row = db.execute(
                text(
                    """
                    select server_id, display_name, description,
                           transport,
                           tags,
                           expected_tools,
                           expected_resources,
                           expected_prompts,
                           state, rate_limit_per_min, daily_budget_cents
                    from mcp_servers where server_id=:server_id
                    """
                ),
                {"server_id": server_id},
            ).mappings().first()
        if not row:
            raise KeyError(f"Unknown MCP server: {server_id}")

        transport_payload = row.get("transport_json") if dialect == "sqlite" else row.get("transport")
        tags = _loads(row.get("tags_json"), []) if dialect == "sqlite" else (row.get("tags") or [])
        expected_tools = _loads(row.get("expected_tools_json"), []) if dialect == "sqlite" else (row.get("expected_tools") or [])
        expected_resources = _loads(row.get("expected_resources_json"), []) if dialect == "sqlite" else (row.get("expected_resources") or [])
        expected_prompts = _loads(row.get("expected_prompts_json"), []) if dialect == "sqlite" else (row.get("expected_prompts") or [])

        return MCPServerConfig(
            server_id=str(row.get("server_id")),
            display_name=str(row.get("display_name") or row.get("server_id")),
            description=row.get("description"),
            transport=_loads(transport_payload, {}) if isinstance(transport_payload, str) else (transport_payload or {}),
            tags=list(tags or []),
            expected_tools=list(expected_tools or []),
            expected_resources=list(expected_resources or []),
            expected_prompts=list(expected_prompts or []),
            rate_limit_per_min=int(row.get("rate_limit_per_min") or 60),
            daily_budget_cents=int(row.get("daily_budget_cents") or 500),
            state=str(row.get("state") or "registered"),
        )

    def get_server(self, db: Session, server_id: str) -> RegistryServer:
        self.ensure_tables(db)
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            row = db.execute(
                text(
                    """
                    select server_id, display_name, description, transport_json,
                           tags_json, expected_tools_json, expected_resources_json, expected_prompts_json,
                           tools_json, resources_json, prompts_json, state, rate_limit_per_min, daily_budget_cents
                    from mcp_servers
                    where server_id=:server_id
                    """
                ),
                {"server_id": server_id},
            ).mappings().first()
        else:
            row = db.execute(
                text(
                    """
                    select server_id, display_name, description, transport, tags, expected_tools, expected_resources, expected_prompts,
                           tools, resources, prompts, state, rate_limit_per_min, daily_budget_cents
                    from mcp_servers
                    where server_id=:server_id
                    """
                ),
                {"server_id": server_id},
            ).mappings().first()
        if not row:
            raise KeyError(f"Unknown MCP server: {server_id}")

        config = self.get_server_config(db, server_id)
        tools_payload = row.get("tools_json") if dialect == "sqlite" else row.get("tools")
        resources_payload = row.get("resources_json") if dialect == "sqlite" else row.get("resources")
        prompts_payload = row.get("prompts_json") if dialect == "sqlite" else row.get("prompts")

        tools_raw = _loads(tools_payload, []) if isinstance(tools_payload, str) else (tools_payload or [])
        resources_raw = _loads(resources_payload, []) if isinstance(resources_payload, str) else (resources_payload or [])
        prompts_raw = _loads(prompts_payload, []) if isinstance(prompts_payload, str) else (prompts_payload or [])

        tools: list[MCPToolSchema] = []
        for item in tools_raw or []:
            if isinstance(item, dict):
                try:
                    tools.append(MCPToolSchema(**item))
                except Exception:
                    continue

        resources = [item for item in (resources_raw or []) if isinstance(item, dict)]
        prompts = [item for item in (prompts_raw or []) if isinstance(item, dict)]

        return RegistryServer(config=config, tools=tools, resources=resources, prompts=prompts)

    def list_servers(self, db: Session, *, user_id: str | None = None) -> list[MCPServerSummary]:
        self.ensure_tables(db)
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if user_id:
            if dialect == "sqlite":
                rows = db.execute(
                    text(
                        """
                        select s.server_id, s.display_name, s.state, s.health_status,
                               s.tools_json, s.resources_json, s.prompts_json
                        from mcp_servers s
                        join mcp_user_servers u on u.server_id = s.server_id
                        where u.user_id = :user_id and u.is_enabled in (true, 1)
                        order by s.display_name asc
                        """
                    ),
                    {"user_id": user_id},
                ).mappings().all()
            else:
                rows = db.execute(
                    text(
                        """
                        select s.server_id, s.display_name, s.state, s.health_status,
                               s.tools, s.resources, s.prompts
                        from mcp_servers s
                        join mcp_user_servers u on u.server_id = s.server_id
                        where u.user_id::text = :user_id and u.is_enabled = true
                        order by s.display_name asc
                        """
                    ),
                    {"user_id": user_id},
                ).mappings().all()
        else:
            if dialect == "sqlite":
                rows = db.execute(
                    text(
                        """
                        select server_id, display_name, state, health_status,
                               tools_json, resources_json, prompts_json
                        from mcp_servers
                        order by display_name asc
                        """
                    )
                ).mappings().all()
            else:
                rows = db.execute(
                    text(
                        """
                        select server_id, display_name, state, health_status,
                               tools, resources, prompts
                        from mcp_servers
                        order by display_name asc
                        """
                    )
                ).mappings().all()

        out: list[MCPServerSummary] = []
        for row in rows:
            if dialect == "sqlite":
                tools_payload = row.get("tools_json")
                resources_payload = row.get("resources_json")
                prompts_payload = row.get("prompts_json")
            else:
                tools_payload = row.get("tools")
                resources_payload = row.get("resources")
                prompts_payload = row.get("prompts")

            tools = _loads(tools_payload, []) if isinstance(tools_payload, str) else (tools_payload or [])
            resources = _loads(resources_payload, []) if isinstance(resources_payload, str) else (resources_payload or [])
            prompts = _loads(prompts_payload, []) if isinstance(prompts_payload, str) else (prompts_payload or [])

            out.append(
                MCPServerSummary(
                    server_id=str(row.get("server_id")),
                    display_name=str(row.get("display_name") or row.get("server_id")),
                    state=str(row.get("state") or "registered"),
                    tools_count=len(tools or []),
                    resources_count=len(resources or []),
                    prompts_count=len(prompts or []),
                    health=None,
                )
            )
        return out

    def set_health(
        self,
        db: Session,
        *,
        server_id: str,
        state: str,
        is_healthy: bool,
        total_calls_delta: int = 0,
        total_errors_delta: int = 0,
        total_cost_delta: float = 0,
    ) -> None:
        self.ensure_tables(db)
        dialect = db.bind.dialect.name if db.bind is not None else ""
        if dialect == "sqlite":
            row = db.execute(
                text(
                    "select total_calls, total_errors, total_cost_cents, consecutive_failures from mcp_servers where server_id=:server_id"
                ),
                {"server_id": server_id},
            ).mappings().first()
            if not row:
                return
            consecutive = 0 if is_healthy else int(row.get("consecutive_failures") or 0) + 1
            db.execute(
                text(
                    """
                    update mcp_servers
                    set state=:state,
                        health_status=:health_status,
                        last_health_check_at=:now,
                        consecutive_failures=:consecutive,
                        total_calls=:total_calls,
                        total_errors=:total_errors,
                        total_cost_cents=:total_cost,
                        updated_at=:now
                    where server_id=:server_id
                    """
                ),
                {
                    "server_id": server_id,
                    "state": state,
                    "health_status": "healthy" if is_healthy else "unhealthy",
                    "now": datetime.utcnow().isoformat(),
                    "consecutive": consecutive,
                    "total_calls": int(row.get("total_calls") or 0) + int(total_calls_delta),
                    "total_errors": int(row.get("total_errors") or 0) + int(total_errors_delta),
                    "total_cost": float(row.get("total_cost_cents") or 0.0) + float(total_cost_delta),
                },
            )
            db.commit()
            return

        db.execute(
            text(
                """
                update mcp_servers
                set state=:state,
                    health_status=:health_status,
                    last_health_check_at=now(),
                    consecutive_failures=case when :is_healthy then 0 else coalesce(consecutive_failures, 0) + 1 end,
                    total_calls=coalesce(total_calls, 0) + :calls,
                    total_errors=coalesce(total_errors, 0) + :errors,
                    total_cost_cents=coalesce(total_cost_cents, 0) + :cost,
                    updated_at=now()
                where server_id=:server_id
                """
            ),
            {
                "server_id": server_id,
                "state": state,
                "health_status": "healthy" if is_healthy else "unhealthy",
                "is_healthy": bool(is_healthy),
                "calls": int(total_calls_delta),
                "errors": int(total_errors_delta),
                "cost": float(total_cost_delta),
            },
        )
        db.commit()
