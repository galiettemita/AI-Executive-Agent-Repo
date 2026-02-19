from __future__ import annotations

import asyncio
from typing import Any

from app.blueprint.mcp.contracts import (
    MCPContentBlock,
    MCPServerManifest,
    MCPToolResult,
    MCPToolSchema,
    MCPTransportConfig,
    MCPTransportType,
)

_WAVE1_MOCK_TOOLS: dict[str, list[MCPToolSchema]] = {
    "google-calendar-mcp": [
        MCPToolSchema(
            name="calendar.list",
            description="List upcoming calendar events",
            inputSchema={
                "type": "object",
                "properties": {
                    "start": {"type": "string"},
                    "end": {"type": "string"},
                },
            },
        ),
        MCPToolSchema(
            name="calendar.create",
            description="Create a calendar event",
            inputSchema={
                "type": "object",
                "properties": {
                    "title": {"type": "string"},
                    "start": {"type": "string"},
                    "end": {"type": "string"},
                },
                "required": ["title", "start", "end"],
            },
        ),
        MCPToolSchema(
            name="calendar.update",
            description="Update an existing calendar event",
            inputSchema={
                "type": "object",
                "properties": {
                    "event_id": {"type": "string"},
                    "title": {"type": "string"},
                },
                "required": ["event_id"],
            },
        ),
        MCPToolSchema(
            name="calendar.delete",
            description="Delete calendar event",
            inputSchema={
                "type": "object",
                "properties": {"event_id": {"type": "string"}},
                "required": ["event_id"],
            },
        ),
    ],
    "google-drive-mcp": [
        MCPToolSchema(
            name="drive.search",
            description="Search drive files",
            inputSchema={
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
        ),
        MCPToolSchema(
            name="drive.get_file",
            description="Get drive file metadata",
            inputSchema={
                "type": "object",
                "properties": {"file_id": {"type": "string"}},
                "required": ["file_id"],
            },
        ),
        MCPToolSchema(
            name="drive.list_recent",
            description="List recent drive files",
            inputSchema={"type": "object", "properties": {"limit": {"type": "integer"}}},
        ),
    ],
    "gmail-mcp": [
        MCPToolSchema(
            name="gmail.search",
            description="Search gmail inbox",
            inputSchema={
                "type": "object",
                "properties": {"query": {"type": "string"}},
                "required": ["query"],
            },
        ),
        MCPToolSchema(
            name="gmail.get_message",
            description="Get email message by id",
            inputSchema={
                "type": "object",
                "properties": {"message_id": {"type": "string"}},
                "required": ["message_id"],
            },
        ),
        MCPToolSchema(
            name="gmail.send",
            description="Send email",
            inputSchema={
                "type": "object",
                "properties": {
                    "to": {"type": "string"},
                    "subject": {"type": "string"},
                    "body": {"type": "string"},
                },
                "required": ["to", "subject", "body"],
            },
        ),
    ],
    "notion-mcp": [
        MCPToolSchema(
            name="notion.search",
            description="Search Notion pages",
            inputSchema={"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]},
        ),
        MCPToolSchema(
            name="notion.get_page",
            description="Get Notion page",
            inputSchema={"type": "object", "properties": {"page_id": {"type": "string"}}, "required": ["page_id"]},
        ),
        MCPToolSchema(
            name="notion.update_page",
            description="Update Notion page",
            inputSchema={
                "type": "object",
                "properties": {
                    "page_id": {"type": "string"},
                    "content": {"type": "string"},
                },
                "required": ["page_id", "content"],
            },
        ),
    ],
    "todoist-mcp": [
        MCPToolSchema(
            name="todoist.list_tasks",
            description="List tasks",
            inputSchema={"type": "object", "properties": {"project": {"type": "string"}}},
        ),
        MCPToolSchema(
            name="todoist.create_task",
            description="Create task",
            inputSchema={
                "type": "object",
                "properties": {"content": {"type": "string"}},
                "required": ["content"],
            },
        ),
        MCPToolSchema(
            name="todoist.complete_task",
            description="Complete task",
            inputSchema={
                "type": "object",
                "properties": {"task_id": {"type": "string"}},
                "required": ["task_id"],
            },
        ),
    ],
    "brave-search-mcp": [
        MCPToolSchema(
            name="brave.search",
            description="Web search",
            inputSchema={"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]},
        ),
        MCPToolSchema(
            name="brave.news",
            description="News search",
            inputSchema={"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]},
        ),
        MCPToolSchema(
            name="brave.images",
            description="Image search",
            inputSchema={"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]},
        ),
    ],
    "github-mcp": [
        MCPToolSchema(
            name="github.list_repos",
            description="List repositories",
            inputSchema={"type": "object", "properties": {"owner": {"type": "string"}}},
        ),
        MCPToolSchema(
            name="github.search_issues",
            description="Search issues",
            inputSchema={"type": "object", "properties": {"query": {"type": "string"}}, "required": ["query"]},
        ),
        MCPToolSchema(
            name="github.create_issue",
            description="Create issue",
            inputSchema={
                "type": "object",
                "properties": {
                    "repo": {"type": "string"},
                    "title": {"type": "string"},
                },
                "required": ["repo", "title"],
            },
        ),
    ],
    "apple-reminders-mcp": [
        MCPToolSchema(
            name="reminders.list",
            description="List reminders",
            inputSchema={"type": "object", "properties": {"completed": {"type": "boolean"}}},
        ),
        MCPToolSchema(
            name="reminders.create",
            description="Create reminder",
            inputSchema={
                "type": "object",
                "properties": {"title": {"type": "string"}},
                "required": ["title"],
            },
        ),
        MCPToolSchema(
            name="reminders.complete",
            description="Complete reminder",
            inputSchema={
                "type": "object",
                "properties": {"reminder_id": {"type": "string"}},
                "required": ["reminder_id"],
            },
        ),
    ],
}

_WAVE2_MOCK_TOOLS: dict[str, list[MCPToolSchema]] = {
    "slack-mcp": [
        MCPToolSchema(name="channels.list", description="List Slack channels"),
        MCPToolSchema(name="messages.search", description="Search Slack messages"),
        MCPToolSchema(name="messages.send", description="Send Slack message"),
    ],
    "outlook-mcp": [
        MCPToolSchema(name="mail.search", description="Search Outlook mail"),
        MCPToolSchema(name="calendar.list", description="List Outlook calendar events"),
        MCPToolSchema(name="mail.send", description="Send Outlook mail"),
    ],
    "teams-mcp": [
        MCPToolSchema(name="teams.list", description="List Teams"),
        MCPToolSchema(name="channels.list", description="List Teams channels"),
        MCPToolSchema(name="messages.send", description="Send Teams message"),
    ],
    "linear-mcp": [
        MCPToolSchema(name="issues.list", description="List Linear issues"),
        MCPToolSchema(name="issues.create", description="Create Linear issue"),
        MCPToolSchema(name="issues.update", description="Update Linear issue"),
    ],
    "asana-mcp": [
        MCPToolSchema(name="tasks.list", description="List Asana tasks"),
        MCPToolSchema(name="tasks.create", description="Create Asana task"),
        MCPToolSchema(name="tasks.update", description="Update Asana task"),
    ],
    "discord-mcp": [
        MCPToolSchema(name="channels.list", description="List Discord channels"),
        MCPToolSchema(name="messages.send", description="Send Discord message"),
    ],
    "whatsapp-business-mcp": [
        MCPToolSchema(name="templates.list", description="List WhatsApp templates"),
        MCPToolSchema(name="messages.send", description="Send WhatsApp message"),
    ],
}

_WAVE3_MOCK_TOOLS: dict[str, list[MCPToolSchema]] = {
    "stripe-mcp": [
        MCPToolSchema(name="customers.list", description="List Stripe customers"),
        MCPToolSchema(name="invoices.create", description="Create Stripe invoice"),
        MCPToolSchema(name="payments.create", description="Create Stripe payment"),
    ],
    "quickbooks-mcp": [
        MCPToolSchema(name="invoices.create", description="Create QuickBooks invoice"),
        MCPToolSchema(name="payments.create", description="Create QuickBooks payment"),
        MCPToolSchema(name="reports.get", description="Fetch QuickBooks report"),
    ],
    "hubspot-mcp": [
        MCPToolSchema(name="contacts.search", description="Search HubSpot contacts"),
        MCPToolSchema(name="deals.create", description="Create HubSpot deal"),
        MCPToolSchema(name="deals.update", description="Update HubSpot deal"),
    ],
    "salesforce-mcp": [
        MCPToolSchema(name="accounts.search", description="Search Salesforce accounts"),
        MCPToolSchema(name="opportunities.update", description="Update Salesforce opportunity"),
        MCPToolSchema(name="leads.create", description="Create Salesforce lead"),
    ],
    "google-sheets-mcp": [
        MCPToolSchema(name="sheets.read", description="Read Google Sheet"),
        MCPToolSchema(name="sheets.write", description="Write Google Sheet"),
        MCPToolSchema(name="sheets.append", description="Append to Google Sheet"),
    ],
    "airtable-mcp": [
        MCPToolSchema(name="records.search", description="Search Airtable records"),
        MCPToolSchema(name="records.create", description="Create Airtable record"),
        MCPToolSchema(name="records.update", description="Update Airtable record"),
    ],
    "jira-mcp": [
        MCPToolSchema(name="issues.search", description="Search Jira issues"),
        MCPToolSchema(name="issues.create", description="Create Jira issue"),
        MCPToolSchema(name="issues.update", description="Update Jira issue"),
    ],
    "sentry-mcp": [
        MCPToolSchema(name="issues.list", description="List Sentry issues"),
        MCPToolSchema(name="issues.assign", description="Assign Sentry issue"),
        MCPToolSchema(name="projects.list", description="List Sentry projects"),
    ],
}

_WAVE4_MOCK_TOOLS: dict[str, list[MCPToolSchema]] = {
    "google-maps-mcp": [
        MCPToolSchema(name="places.search", description="Search places"),
        MCPToolSchema(name="routes.get", description="Get route"),
        MCPToolSchema(name="eta.get", description="Get ETA"),
    ],
    "uber-lyft-mcp": [
        MCPToolSchema(name="rides.estimate", description="Estimate ride"),
        MCPToolSchema(name="rides.request", description="Request ride"),
        MCPToolSchema(name="rides.status", description="Ride status"),
    ],
    "opentable-resy-mcp": [
        MCPToolSchema(name="restaurants.search", description="Search restaurants"),
        MCPToolSchema(name="reservations.create", description="Create reservation"),
        MCPToolSchema(name="reservations.cancel", description="Cancel reservation"),
    ],
    "homeassistant-mcp": [
        MCPToolSchema(name="devices.list", description="List smart home devices"),
        MCPToolSchema(name="devices.command", description="Send device command"),
        MCPToolSchema(name="scenes.activate", description="Activate scene"),
    ],
    "spotify-mcp": [
        MCPToolSchema(name="playlists.list", description="List Spotify playlists"),
        MCPToolSchema(name="playback.start", description="Start playback"),
        MCPToolSchema(name="playback.pause", description="Pause playback"),
    ],
    "evernote-mcp": [
        MCPToolSchema(name="notes.search", description="Search Evernote notes"),
        MCPToolSchema(name="notes.create", description="Create Evernote note"),
        MCPToolSchema(name="notes.update", description="Update Evernote note"),
    ],
    "dropbox-mcp": [
        MCPToolSchema(name="files.search", description="Search Dropbox files"),
        MCPToolSchema(name="files.get", description="Get Dropbox file"),
        MCPToolSchema(name="files.upload", description="Upload Dropbox file"),
    ],
}


def build_echo_manifest(server_id: str = "mcp-echo") -> MCPServerManifest:
    return MCPServerManifest(
        server_id=server_id,
        display_name="Echo MCP",
        description="Deterministic echo server for tests",
        transport=MCPTransportConfig(type=MCPTransportType.STDIO, command="mock://echo"),
        expected_tools=["echo"],
        expected_resources=[],
        expected_prompts=[],
        tags=["test", "mock"],
    )


def build_error_manifest(server_id: str = "mcp-error") -> MCPServerManifest:
    return MCPServerManifest(
        server_id=server_id,
        display_name="Error MCP",
        description="Always returns error",
        transport=MCPTransportConfig(type=MCPTransportType.STDIO, command="mock://error"),
        expected_tools=["explode"],
        tags=["test", "mock"],
    )


def build_slow_manifest(server_id: str = "mcp-slow") -> MCPServerManifest:
    return MCPServerManifest(
        server_id=server_id,
        display_name="Slow MCP",
        description="Sleeps before responding",
        transport=MCPTransportConfig(type=MCPTransportType.STDIO, command="mock://slow", timeout_ms=800),
        expected_tools=["sleep"],
        tags=["test", "mock"],
    )


def mock_tools_for(server_id: str) -> list[MCPToolSchema]:
    if server_id == "mcp-echo":
        return [
            MCPToolSchema(
                name="echo",
                description="Echoes text",
                inputSchema={
                    "type": "object",
                    "properties": {"text": {"type": "string"}},
                    "required": ["text"],
                },
            )
        ]
    if server_id == "mcp-error":
        return [
            MCPToolSchema(
                name="explode",
                description="Always errors",
                inputSchema={"type": "object", "properties": {}},
            )
        ]
    if server_id == "mcp-slow":
        return [
            MCPToolSchema(
                name="sleep",
                description="Sleeps and responds",
                inputSchema={
                    "type": "object",
                    "properties": {"ms": {"type": "integer", "minimum": 1}},
                },
            )
        ]
    if server_id in _WAVE1_MOCK_TOOLS:
        return list(_WAVE1_MOCK_TOOLS[server_id])
    if server_id in _WAVE2_MOCK_TOOLS:
        return list(_WAVE2_MOCK_TOOLS[server_id])
    if server_id in _WAVE3_MOCK_TOOLS:
        return list(_WAVE3_MOCK_TOOLS[server_id])
    if server_id in _WAVE4_MOCK_TOOLS:
        return list(_WAVE4_MOCK_TOOLS[server_id])
    return []


async def dispatch_mock_tool(server_id: str, tool_name: str, arguments: dict[str, Any]) -> MCPToolResult:
    if server_id == "mcp-echo" and tool_name == "echo":
        text = str(arguments.get("text") or "")
        return MCPToolResult(
            server_id=server_id,
            content=[MCPContentBlock(type="text", text=text)],
            is_error=False,
            latency_ms=5,
        )

    if server_id == "mcp-error":
        return MCPToolResult(
            server_id=server_id,
            content=[MCPContentBlock(type="text", text="forced error")],
            is_error=True,
            latency_ms=3,
        )

    if server_id == "mcp-slow" and tool_name == "sleep":
        sleep_ms = max(1, int(arguments.get("ms") or 50))
        await asyncio.sleep(sleep_ms / 1000)
        return MCPToolResult(
            server_id=server_id,
            content=[MCPContentBlock(type="text", text=f"slept:{sleep_ms}")],
            is_error=False,
            latency_ms=sleep_ms,
        )

    if (
        server_id in _WAVE1_MOCK_TOOLS
        or server_id in _WAVE2_MOCK_TOOLS
        or server_id in _WAVE3_MOCK_TOOLS
        or server_id in _WAVE4_MOCK_TOOLS
    ):
        return MCPToolResult(
            server_id=server_id,
            content=[
                MCPContentBlock(
                    type="text",
                    text=str(
                        {
                            "mock": True,
                            "server_id": server_id,
                            "tool": tool_name,
                            "arguments": arguments,
                            "status": "ok",
                        }
                    ),
                )
            ],
            is_error=False,
            latency_ms=8,
        )

    return MCPToolResult(
        server_id=server_id,
        content=[MCPContentBlock(type="text", text=f"unknown mock tool {tool_name}")],
        is_error=True,
        latency_ms=1,
    )
