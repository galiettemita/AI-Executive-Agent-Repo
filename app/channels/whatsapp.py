# backend/app/channels/whatsapp.py

import requests
import logging
from typing import Dict, Any, Optional
from app.core.config import settings

logger = logging.getLogger(__name__)

WHATSAPP_TOKEN = settings.WHATSAPP_TOKEN
WHATSAPP_PHONE_NUMBER_ID = settings.WHATSAPP_PHONE_NUMBER_ID

GRAPH_URL = "https://graph.facebook.com/v20.0"


def normalize_whatsapp_webhook(payload: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    """
    Returns a normalized event dict:
    {
      "external_id": "...",
      "from": "+15551234567",
      "text": "hello"
    }
    Returns None if payload doesn't contain a user message.
    """
    try:
        logger.debug("Normalizing WhatsApp webhook payload (keys=%s)", list(payload.keys()) if isinstance(payload, dict) else "unknown")
        
        # Check if payload has expected structure
        if "entry" not in payload:
            logger.warning("Payload missing 'entry' field")
            return None
        
        if not payload["entry"]:
            logger.warning("Payload 'entry' is empty")
            return None
        
        entry = payload["entry"][0]
        
        if "changes" not in entry:
            logger.warning("Entry missing 'changes' field")
            return None
        
        if not entry["changes"]:
            logger.warning("Entry 'changes' is empty")
            return None
        
        change = entry["changes"][0]
        value = change["value"]
        
        # Check for statuses (message delivery updates) - ignore these
        if "statuses" in value:
            logger.info("Ignoring status update webhook")
            return None
        
        messages = value.get("messages")
        if not messages:
            logger.info("No messages in webhook payload")
            return None
        
        msg = messages[0]
        msg_type = msg.get("type")
        
        logger.info("WhatsApp message type: %s", msg_type)
        
        if msg_type != "text":
            logger.info("Ignoring non-text message of type: %s", msg_type)
            return None
        
        external_id = msg["id"]
        from_phone = msg["from"]  # WhatsApp provides phone without '+'
        text = msg["text"]["body"]
        
        result = {
            "external_id": external_id,
            "from": f"+{from_phone}",
            "text": text,
        }
        
        logger.info("Successfully normalized WhatsApp message: id=%s", result.get("external_id"))
        return result
        
    except KeyError as e:
        logger.error("KeyError while normalizing webhook: %s", e)
        return None
    except Exception as e:
        logger.error("Unexpected error normalizing webhook: %s", e)
        logger.exception("Full traceback:")
        return None


def send_whatsapp_text(to_phone_e164: str, text: str) -> None:
    """
    Sends a text message back to WhatsApp.
    Requires WHATSAPP_TOKEN + WHATSAPP_PHONE_NUMBER_ID.
    If not configured, silently no-ops (useful for local testing).
    """
    if not WHATSAPP_TOKEN or not WHATSAPP_PHONE_NUMBER_ID:
        logger.warning("WhatsApp credentials not configured. Cannot send message.")
        return
    
    try:
        url = f"{GRAPH_URL}/{WHATSAPP_PHONE_NUMBER_ID}/messages"
        headers = {
            "Authorization": f"Bearer {WHATSAPP_TOKEN}",
            "Content-Type": "application/json",
        }
        data = {
            "messaging_product": "whatsapp",
            "to": to_phone_e164.replace("+", ""),
            "type": "text",
            "text": {"body": text[:4096]},
        }
        
        masked = f"{to_phone_e164[:3]}***{to_phone_e164[-2:]}" if to_phone_e164 else "unknown"
        logger.info("Sending WhatsApp message to %s", masked)
        
        response = requests.post(url, headers=headers, json=data, timeout=20)
        
        logger.info("WhatsApp API response status: %s", response.status_code)
        
        if response.status_code != 200:
            logger.error("Failed to send WhatsApp message (status=%s)", response.status_code)
        else:
            logger.info("WhatsApp message sent successfully")
            
    except requests.exceptions.RequestException as e:
        logger.error("Error sending WhatsApp message: %s", e)
        logger.exception("Full traceback:")
    except Exception as e:
        logger.error("Unexpected error sending WhatsApp message: %s", e)
        logger.exception("Full traceback:")
