# app/services/voice_scenarios.py

from __future__ import annotations

from typing import Dict


SCENARIOS: Dict[str, Dict[str, str]] = {
    "restaurant_reservation": {
        "title": "Restaurant reservation",
        "goal": "Make a reservation with the desired party size, date, time, and name.",
        "required_fields": "restaurant_name, party_size, date, time, name, phone",
    },
    "appointment_scheduling": {
        "title": "Appointment scheduling",
        "goal": "Schedule or reschedule an appointment at the requested time and date.",
        "required_fields": "business_name, service_type, date, time, name, phone",
    },
    "customer_service": {
        "title": "Customer service inquiry",
        "goal": "Resolve an issue, obtain a reference number, or clarify account status.",
        "required_fields": "company_name, issue_summary, account_identifier",
    },
    "bill_negotiation": {
        "title": "Bill negotiation",
        "goal": "Negotiate a lower rate or secure a discount or payment plan.",
        "required_fields": "provider_name, account_identifier, target_rate, reason",
    },
    "rsvp": {
        "title": "RSVP response",
        "goal": "Respond to an invitation with attendance and any notes.",
        "required_fields": "event_name, attendance, guest_count, notes",
    },
}


def get_scenario(purpose: str | None) -> Dict[str, str]:
    if not purpose:
        return {
            "title": "General call",
            "goal": "Handle the call professionally and complete the user's request.",
            "required_fields": "name, phone, request",
        }
    return SCENARIOS.get(purpose, {
        "title": "General call",
        "goal": "Handle the call professionally and complete the user's request.",
        "required_fields": "name, phone, request",
    })
