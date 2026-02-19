from __future__ import annotations

from app.api.routes import v1_gateway


def test_notification_preference_command_stop_morning_briefings():
    parsed = v1_gateway._notification_preference_command("Stop sending me morning briefings")
    assert parsed is not None
    patch, reply = parsed
    assert patch.get("morning_briefings_enabled") is False
    assert "stop sending morning briefings" in reply.lower()


def test_notification_preference_command_pause_proactive():
    parsed = v1_gateway._notification_preference_command("I don't want proactive messages")
    assert parsed is not None
    patch, reply = parsed
    assert patch.get("proactive_notifications_enabled") is False
    assert patch.get("proactive_disabled") is True
    assert "pause proactive messages" in reply.lower()


def test_provisioning_decline_command_detection():
    assert v1_gateway._is_provisioning_decline_command("not now")
    assert v1_gateway._is_provisioning_decline_command("maybe later, skip for now")
    assert not v1_gateway._is_provisioning_decline_command("yes connect it")
