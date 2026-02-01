# backend/app/channels/whatsapp.py

import os
import requests
import logging
from typing import Dict, Any, Optional

logger = logging.getLogger(__name__)

WHATSAPP_TOKEN = os.getenv("WHATSAPP_TOKEN", "")
WHATSAPP_PHONE_NUMBER_ID = os.getenv("WHATSAPP_PHONE_NUMBER_ID", "")

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
        logger.debug(f"Normalizing WhatsApp webhook payload: {payload}")
        
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
        
        logger.info(f"Message type: {msg_type}")
        
        if msg_type != "text":
            logger.info(f"Ignoring non-text message of type: {msg_type}")
            return None
        
        external_id = msg["id"]
        from_phone = msg["from"]  # WhatsApp provides phone without '+'
        text = msg["text"]["body"]
        
        result = {
            "external_id": external_id,
            "from": f"+{from_phone}",
            "text": text,
        }
        
        logger.info(f"Successfully normalized message: {result}")
        return result
        
    except KeyError as e:
        logger.error(f"KeyError while normalizing webhook: {e}")
        logger.error(f"Payload structure: {payload}")
        return None
    except Exception as e:
        logger.error(f"Unexpected error normalizing webhook: {e}")
        logger.exception("Full traceback:")
        return None


def send_whatsapp_text(to_phone_e164: str, text: str) -> None:
    """
    Sends a text message back to WhatsApp.
    Requires WHATSAPP_TOKEN + WHATSAPP_PHONE_NUMBER_ID.
    If not configured, silently no-ops (useful for local testing).
    """
    if not WHATSAPP_TOKEN or not WHATSAPP_PHONE_NUMBER_ID:
        logger.warning("⚠️  WhatsApp credentials not configured. Cannot send message.")
        logger.warning(f"WHATSAPP_TOKEN present: {bool(WHATSAPP_TOKEN)}")
        logger.warning(f"WHATSAPP_PHONE_NUMBER_ID present: {bool(WHATSAPP_PHONE_NUMBER_ID)}")
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
        
        logger.info(f"Sending WhatsApp message to {to_phone_e164}")
        logger.debug(f"Message payload: {data}")
        
        response = requests.post(url, headers=headers, json=data, timeout=20)
        
        logger.info(f"WhatsApp API response status: {response.status_code}")
        logger.debug(f"WhatsApp API response: {response.text}")
        
        if response.status_code != 200:
            logger.error(f"Failed to send WhatsApp message: {response.text}")
        else:
            logger.info("✅ WhatsApp message sent successfully")
            
    except requests.exceptions.RequestException as e:
        logger.error(f"Error sending WhatsApp message: {e}")
        logger.exception("Full traceback:")
    except Exception as e:
        logger.error(f"Unexpected error sending WhatsApp message: {e}")
        logger.exception("Full traceback:")