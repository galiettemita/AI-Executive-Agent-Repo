from __future__ import annotations

import html
import io
from pathlib import Path
from typing import Optional
from urllib.parse import quote

from fastapi import APIRouter, Depends, HTTPException, Request
from fastapi.responses import HTMLResponse, Response
from sqlalchemy.orm import Session

from app.api.deps import get_db, get_or_create_user
from app.core.config import settings
from app.middleware.rate_limiter import rate_limit_webhook
from app.schemas.public_site import LocationShareRequest, LocationIpRequest
from app.services.location_service import build_location_patch, resolve_ip_location
from app.services.profile_service import update_profile


router = APIRouter(tags=["public"])

ROOT_DIR = Path(__file__).resolve().parents[3]
DOCS_DIR = ROOT_DIR / "docs"


def _normalize_whatsapp_number(value: str) -> str:
    digits = "".join(ch for ch in value if ch.isdigit())
    return digits


def _whatsapp_link() -> Optional[str]:
    if not settings.WHATSAPP_PUBLIC_NUMBER:
        return None
    number = _normalize_whatsapp_number(settings.WHATSAPP_PUBLIC_NUMBER)
    if not number:
        return None
    return f"https://wa.me/{number}?text={quote('Hi! I want to try Executive AI Agent.')}"


def _render_markdown_page(title: str, markdown_path: Path) -> HTMLResponse:
    if not markdown_path.exists():
        return HTMLResponse(
            content=f"<h1>{title}</h1><p>Document not available.</p>",
            status_code=404,
        )
    content = markdown_path.read_text(encoding="utf-8", errors="ignore")
    escaped = html.escape(content)
    html_body = f"""
    <!doctype html>
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width,initial-scale=1" />
        <title>{title}</title>
        <style>
          body {{
            font-family: "IBM Plex Sans", system-ui, sans-serif;
            background: #0f1419;
            color: #f5f7fa;
            padding: 32px;
          }}
          a {{ color: #7dd3fc; }}
          pre {{
            white-space: pre-wrap;
            background: #151b22;
            padding: 24px;
            border-radius: 16px;
            border: 1px solid #233042;
          }}
        </style>
      </head>
      <body>
        <h1>{title}</h1>
        <pre>{escaped}</pre>
      </body>
    </html>
    """
    return HTMLResponse(content=html_body)


@router.get("/", response_class=HTMLResponse)
def landing_page(request: Request) -> HTMLResponse:
    wa_link = _whatsapp_link()
    qr_src = "/public/qr"
    if wa_link:
        qr_src = f"/public/qr?text={quote(wa_link)}"

    support_email = settings.PUBLIC_SITE_SUPPORT_EMAIL
    site_name = settings.PUBLIC_SITE_NAME
    tagline = settings.PUBLIC_SITE_TAGLINE

    html_body = f"""
    <!doctype html>
    <html lang="en">
      <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width,initial-scale=1" />
        <title>{site_name}</title>
        <link rel="preconnect" href="https://fonts.googleapis.com">
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
        <link href="https://fonts.googleapis.com/css2?family=Fraunces:wght@400;600;700&family=Space+Grotesk:wght@400;500;600&display=swap" rel="stylesheet">
        <style>
          :root {{
            --ink: #0b0f14;
            --mist: #f6f4ff;
            --accent: #0ea5e9;
            --accent-2: #9333ea;
            --panel: rgba(255, 255, 255, 0.75);
            --shadow: 0 20px 60px rgba(15, 23, 42, 0.25);
          }}
          * {{ box-sizing: border-box; }}
          body {{
            margin: 0;
            font-family: "Space Grotesk", system-ui, sans-serif;
            background: radial-gradient(circle at top, #fef9c3, #e0e7ff 35%, #dbeafe 70%, #e2e8f0 100%);
            color: var(--ink);
          }}
          header {{
            padding: 32px 24px 10px;
          }}
          .hero {{
            max-width: 1100px;
            margin: 0 auto;
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
            gap: 32px;
            align-items: center;
            padding: 32px 24px 64px;
          }}
          .hero-card {{
            background: var(--panel);
            border-radius: 28px;
            padding: 32px;
            box-shadow: var(--shadow);
            backdrop-filter: blur(16px);
          }}
          h1 {{
            font-family: "Fraunces", serif;
            font-size: clamp(2.4rem, 4vw, 4rem);
            margin: 0 0 12px;
          }}
          h2 {{
            font-family: "Fraunces", serif;
            font-size: 1.8rem;
            margin-top: 0;
          }}
          p {{
            line-height: 1.6;
            margin: 0 0 16px;
          }}
          .cta {{
            display: inline-flex;
            align-items: center;
            gap: 10px;
            background: linear-gradient(120deg, var(--accent), var(--accent-2));
            color: white;
            border-radius: 999px;
            padding: 12px 22px;
            text-decoration: none;
            font-weight: 600;
          }}
          .qr {{
            display: flex;
            flex-direction: column;
            gap: 12px;
            align-items: center;
            text-align: center;
          }}
          .qr img {{
            width: 180px;
            height: 180px;
            border-radius: 18px;
            background: white;
            padding: 8px;
          }}
          .grid {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 20px;
          }}
          .panel {{
            background: rgba(255, 255, 255, 0.7);
            border-radius: 20px;
            padding: 20px;
            box-shadow: 0 10px 30px rgba(30, 41, 59, 0.15);
          }}
          .section {{
            max-width: 1100px;
            margin: 0 auto;
            padding: 24px;
          }}
          .location-box {{
            background: #0f172a;
            color: #e2e8f0;
            border-radius: 20px;
            padding: 24px;
          }}
          .location-box input, .location-box button {{
            padding: 10px 12px;
            border-radius: 10px;
            border: none;
          }}
          .location-box input {{
            width: 100%;
            margin-bottom: 10px;
          }}
          .location-box button {{
            margin-right: 8px;
            cursor: pointer;
          }}
          .location-status {{
            margin-top: 10px;
            font-size: 0.9rem;
          }}
          footer {{
            padding: 40px 24px 60px;
            text-align: center;
            color: #475569;
          }}
          a.link {{
            color: #0f172a;
            font-weight: 600;
            text-decoration: none;
          }}
        </style>
      </head>
      <body>
        <header>
          <div class="section">
            <strong>{site_name}</strong>
          </div>
        </header>
        <section class="hero">
          <div class="hero-card">
            <h1>{tagline}</h1>
            <p>Coordinate travel, messages, calendars, and life logistics directly in WhatsApp — with approvals, audit trails, and full control.</p>
            <div style="margin: 24px 0;">
              {f'<a class="cta" href="{wa_link}" target="_blank" rel="noopener">Start on WhatsApp →</a>' if wa_link else '<span class="cta">WhatsApp link coming soon</span>'}
            </div>
            <p style="font-size: 0.95rem; color: #475569;">Privacy-first by design. You control what the assistant can see and do.</p>
          </div>
          <div class="hero-card qr">
            <h2>Scan to start</h2>
            <img src="{qr_src}" alt="WhatsApp QR Code" />
            <p>Open your camera or WhatsApp scanner.</p>
          </div>
        </section>

        <section class="section">
          <h2>What it handles</h2>
          <div class="grid">
            <div class="panel">Smart messaging, approvals, and proactive alerts.</div>
            <div class="panel">Calendar scheduling, briefs, and follow-ups.</div>
            <div class="panel">Email intelligence and draft responses.</div>
            <div class="panel">Travel planning, bookings, and receipts.</div>
          </div>
        </section>

        <section class="section">
          <h2>Share your location (optional)</h2>
          <div class="location-box">
            <p>We use location only to improve recommendations (weather, travel time). You can revoke anytime.</p>
            <label for="userId">WhatsApp number (E.164, e.g. +15551234567)</label>
            <input id="userId" placeholder="+1..." />
            <label for="manualCity">City (optional)</label>
            <input id="manualCity" placeholder="San Francisco, CA" />
            <div style="margin: 12px 0;">
              <label><input type="checkbox" id="consent" /> I consent to storing my location.</label>
            </div>
            <div>
              <button id="geoBtn">Share precise location</button>
              <button id="ipBtn">Use IP-based location</button>
              <button id="manualBtn">Save city only</button>
            </div>
            <div class="location-status" id="locationStatus"></div>
          </div>
        </section>

        <section class="section">
          <h2>Trust & safety</h2>
          <div class="grid">
            <div class="panel">Every action is logged and permissioned.</div>
            <div class="panel">You control integrations and data retention.</div>
            <div class="panel">Secure by default with encryption and audit trails.</div>
            <div class="panel">
              <a class="link" href="/legal/privacy">Privacy Policy</a> ·
              <a class="link" href="/legal/terms">Terms of Service</a>
            </div>
          </div>
        </section>

        <footer>
          Questions? Email <a class="link" href="mailto:{support_email}">{support_email}</a>
        </footer>

        <script>
          const statusEl = document.getElementById("locationStatus");
          const userIdEl = document.getElementById("userId");
          const consentEl = document.getElementById("consent");
          const manualCityEl = document.getElementById("manualCity");

          function setStatus(msg) {{
            statusEl.textContent = msg;
          }}

          function requireConsent() {{
            if (!consentEl.checked) {{
              setStatus("Please check the consent box first.");
              return false;
            }}
            if (!userIdEl.value.trim()) {{
              setStatus("Please add your WhatsApp number first.");
              return false;
            }}
            return true;
          }}

          async function sendLocation(payload, endpoint="/public/location") {{
            const resp = await fetch(endpoint, {{
              method: "POST",
              headers: {{ "Content-Type": "application/json" }},
              body: JSON.stringify(payload),
            }});
            const data = await resp.json();
            if (resp.ok) {{
              setStatus("Location saved. Thanks!");
            }} else {{
              setStatus(data?.detail || data?.error || "Unable to save location.");
            }}
          }}

          document.getElementById("geoBtn").addEventListener("click", () => {{
            if (!requireConsent()) return;
            if (!navigator.geolocation) {{
              setStatus("Geolocation not supported. Try IP-based or city-only.");
              return;
            }}
            setStatus("Requesting location...");
            navigator.geolocation.getCurrentPosition(
              (pos) => {{
                sendLocation({{
                  user_id: userIdEl.value.trim(),
                  consent: true,
                  source: "browser_geolocation",
                  latitude: pos.coords.latitude,
                  longitude: pos.coords.longitude,
                  accuracy_m: pos.coords.accuracy,
                }});
              }},
              () => setStatus("Permission denied. Try IP-based or city-only."),
              {{ enableHighAccuracy: true, timeout: 10000 }}
            );
          }});

          document.getElementById("ipBtn").addEventListener("click", () => {{
            if (!requireConsent()) return;
            setStatus("Resolving IP location...");
            sendLocation({{
              user_id: userIdEl.value.trim(),
              consent: true,
              source: "ip",
            }}, "/public/location/ip");
          }});

          document.getElementById("manualBtn").addEventListener("click", () => {{
            if (!requireConsent()) return;
            if (!manualCityEl.value.trim()) {{
              setStatus("Please enter a city to save.");
              return;
            }}
            sendLocation({{
              user_id: userIdEl.value.trim(),
              consent: true,
              source: "manual",
              city: manualCityEl.value.trim(),
              location: manualCityEl.value.trim(),
            }});
          }});
        </script>
      </body>
    </html>
    """
    return HTMLResponse(content=html_body)


@router.get("/legal/privacy", response_class=HTMLResponse)
def privacy_policy():
    return _render_markdown_page("Privacy Policy", DOCS_DIR / "PRIVACY_POLICY.md")


@router.get("/legal/terms", response_class=HTMLResponse)
def terms_of_service():
    return _render_markdown_page("Terms of Service", DOCS_DIR / "LEGAL_TOS.md")


@rate_limit_webhook()
@router.get("/public/qr")
def whatsapp_qr(request: Request, text: Optional[str] = None):
    try:
        import qrcode
    except Exception:
        raise HTTPException(status_code=500, detail="QR generator not installed")

    data = text or _whatsapp_link() or settings.APP_BASE_URL
    if not data:
        raise HTTPException(status_code=400, detail="No QR data available")

    img = qrcode.make(data)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return Response(content=buf.getvalue(), media_type="image/png")


@rate_limit_webhook()
@router.post("/public/location")
def capture_location(request: Request, payload: LocationShareRequest, db: Session = Depends(get_db)):
    if not payload.consent:
        raise HTTPException(status_code=400, detail="Consent required")

    get_or_create_user(db, payload.user_id)
    patch = build_location_patch(
        source=payload.source or "browser_geolocation",
        latitude=payload.latitude,
        longitude=payload.longitude,
        accuracy_m=payload.accuracy_m,
        city=payload.city,
        region=payload.region,
        country=payload.country,
        timezone=payload.timezone,
        ip=payload.ip,
        location_label=payload.location,
    )
    profile = update_profile(db, payload.user_id, patch)
    return {"ok": True, "profile": profile}


@rate_limit_webhook()
@router.post("/public/location/ip")
async def capture_location_from_ip(
    request: Request,
    payload: LocationIpRequest,
    db: Session = Depends(get_db),
):
    if not payload.consent:
        raise HTTPException(status_code=400, detail="Consent required")

    get_or_create_user(db, payload.user_id)
    ip = request.headers.get("X-Forwarded-For", "").split(",")[0].strip() or request.client.host
    resolved = await resolve_ip_location(ip)
    patch = build_location_patch(
        source="ip",
        latitude=resolved.get("latitude"),
        longitude=resolved.get("longitude"),
        city=resolved.get("city"),
        region=resolved.get("region"),
        country=resolved.get("country"),
        timezone=resolved.get("timezone"),
        ip=resolved.get("ip") or ip,
    )
    profile = update_profile(db, payload.user_id, patch)
    return {"ok": True, "profile": profile, "resolved": resolved}
