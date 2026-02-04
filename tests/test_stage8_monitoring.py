"""
Tests for Stage 8: Monitoring/Alerts Workers

Tests the monitoring infrastructure including:
- Scheduled price monitoring jobs
- Notification queue and delivery
- Manual trigger endpoints
"""
import uuid
from datetime import datetime
from unittest.mock import patch, MagicMock

from fastapi.testclient import TestClient
from sqlalchemy.orm import Session

from app.main import app
from app.db.database import SessionLocal
from app.db.models import WatchItem, NotificationQueue
from app.services.notification_delivery import deliver_pending_notifications
from app.services.scheduler import run_price_monitoring


client = TestClient(app)


# ===== NOTIFICATION DELIVERY TESTS =====


def test_notification_queue_model():
    """Test that NotificationQueue model works correctly"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create a watch item first
    watch_item = WatchItem(
        user_id=user_id,
        url="https://example.com/product",
        title_hint="Test Product",
        desired_price=50.0,
    )
    db.add(watch_item)
    db.commit()
    db.refresh(watch_item)

    # Create notification
    notification = NotificationQueue(
        user_id=user_id,
        watch_item_id=watch_item.id,
        event_type="price_drop",
        title="Price dropped",
        message="Test product dropped from $60 to $50",
        deep_link_url="https://example.com/buy",
        prev_price=60.0,
        new_price=50.0,
        currency="USD",
        is_sent=False,
    )
    db.add(notification)
    db.commit()
    db.refresh(notification)

    # Verify
    assert notification.id is not None
    assert notification.is_sent is False
    assert notification.sent_at is None
    assert notification.event_type == "price_drop"

    # Cleanup
    db.delete(notification)
    db.delete(watch_item)
    db.commit()
    db.close()


def test_deliver_pending_notifications_with_mock():
    """Test notification delivery service with mocked WhatsApp API"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create a watch item
    watch_item = WatchItem(
        user_id=user_id,
        url="https://example.com/product",
        title_hint="Test Product",
    )
    db.add(watch_item)
    db.commit()
    db.refresh(watch_item)

    # Create pending notification
    notification = NotificationQueue(
        user_id=user_id,
        watch_item_id=watch_item.id,
        event_type="price_drop",
        title="Price dropped",
        message="Test product dropped in price",
        deep_link_url="https://example.com/buy",
        is_sent=False,
    )
    db.add(notification)
    db.commit()
    db.refresh(notification)

    # Mock WhatsApp API
    with patch("app.services.notification_delivery.send_whatsapp_message") as mock_send:
        mock_send.return_value = True

        # Deliver notifications
        result = deliver_pending_notifications(db)

        # Verify delivery was attempted
        assert mock_send.called
        assert result["sent"] >= 0  # Should have sent at least our notification

        # Verify notification was marked as sent
        db.refresh(notification)
        assert notification.is_sent is True
        assert notification.sent_at is not None

    # Cleanup
    db.delete(notification)
    db.delete(watch_item)
    db.commit()
    db.close()


def test_notification_delivery_skips_already_sent():
    """Test that already-sent notifications are not delivered again"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create a watch item
    watch_item = WatchItem(
        user_id=user_id,
        url="https://example.com/product",
    )
    db.add(watch_item)
    db.commit()
    db.refresh(watch_item)

    # Create already-sent notification
    notification = NotificationQueue(
        user_id=user_id,
        watch_item_id=watch_item.id,
        event_type="price_drop",
        title="Price dropped",
        message="Already sent",
        is_sent=True,
        sent_at=datetime.utcnow(),
    )
    db.add(notification)
    db.commit()

    # Mock WhatsApp API
    with patch("app.services.notification_delivery.send_whatsapp_message") as mock_send:
        result = deliver_pending_notifications(db)

        # Should NOT send already-sent notifications
        assert result["sent"] == 0

    # Cleanup
    db.delete(notification)
    db.delete(watch_item)
    db.commit()
    db.close()


# ===== PRICE MONITORING TESTS =====


def test_price_monitoring_worker_with_mock():
    """Test that price monitoring worker processes watch items correctly"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create a watch item
    watch_item = WatchItem(
        user_id=user_id,
        url="https://example.com/product",
        title_hint="Test Product",
        best_price=100.0,
        desired_price=80.0,
    )
    db.add(watch_item)
    db.commit()
    db.refresh(watch_item)

    # Mock price lookup to return a lower price (price drop scenario)
    mock_offers = [
        {
            "price": 75.0,
            "currency": "USD",
            "retailer": "TestRetailer",
            "offer_url": "https://example.com/buy",
        }
    ]

    with patch("app.services.price_lookup.lookup_price_google_shopping") as mock_lookup:
        mock_lookup.return_value = (
            75.0,  # best_price
            "USD",  # currency
            "TestRetailer",  # best_retailer
            "https://example.com/buy",  # best_offer_url
            {"title": "Test Product"},  # best_meta
            mock_offers,  # offers
        )

        # Run price monitoring
        run_price_monitoring()

        # Verify watch item was updated
        db.refresh(watch_item)
        assert watch_item.last_checked_at is not None
        assert watch_item.best_price == 75.0

        # Verify notifications were queued
        notifications = (
            db.query(NotificationQueue)
            .filter(NotificationQueue.watch_item_id == watch_item.id)
            .all()
        )

        # Should have both price_drop AND target_hit notifications
        event_types = [n.event_type for n in notifications]
        assert "price_drop" in event_types  # Dropped from 100 to 75
        assert "target_hit" in event_types  # Hit target of 80

    # Cleanup
    for notification in notifications:
        db.delete(notification)
    db.delete(watch_item)
    db.commit()
    db.close()


# ===== API ENDPOINT TESTS =====


def test_trigger_price_check_endpoint():
    """Test manual price check trigger endpoint"""
    with patch("app.services.price_lookup.lookup_price_google_shopping") as mock_lookup:
        mock_lookup.return_value = (50.0, "USD", "TestRetailer", "https://example.com", {}, [])

        resp = client.post("/monitoring/trigger/price-check")
        assert resp.status_code == 200
        data = resp.json()
        assert data["ok"] is True


def test_trigger_send_notifications_endpoint():
    """Test manual notification delivery trigger endpoint"""
    with patch("app.services.notification_delivery.send_whatsapp_message") as mock_send:
        mock_send.return_value = True

        resp = client.post("/monitoring/trigger/send-notifications")
        assert resp.status_code == 200
        data = resp.json()
        assert data["ok"] is True


def test_list_notifications_endpoint():
    """Test listing pending notifications via API"""
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # Create watch item
    watch_item = WatchItem(
        user_id=user_id,
        url="https://example.com/product",
    )
    db.add(watch_item)
    db.commit()
    db.refresh(watch_item)

    # Create pending notification
    notification = NotificationQueue(
        user_id=user_id,
        watch_item_id=watch_item.id,
        event_type="price_drop",
        title="Price dropped",
        message="Test notification",
        is_sent=False,
    )
    db.add(notification)
    db.commit()

    # List notifications via API
    resp = client.get(f"/notifications?user_id={user_id}")
    assert resp.status_code == 200
    data = resp.json()
    assert "items" in data
    assert len(data["items"]) > 0

    # Verify notification data
    found = False
    for item in data["items"]:
        if item["id"] == notification.id:
            found = True
            assert item["event_type"] == "price_drop"
            assert item["title"] == "Price dropped"
            break

    assert found, "Created notification should be in API response"

    # Cleanup
    db.delete(notification)
    db.delete(watch_item)
    db.commit()
    db.close()


# ===== INTEGRATION TESTS =====


def test_end_to_end_price_monitoring_flow():
    """
    Test complete flow: price monitoring → notification queue → delivery
    """
    db = SessionLocal()
    user_id = f"test_user_{uuid.uuid4().hex[:8]}"

    # 1. Create watch item with high price
    watch_item = WatchItem(
        user_id=user_id,
        url="https://example.com/product",
        title_hint="Test Product",
        best_price=100.0,
        desired_price=50.0,
    )
    db.add(watch_item)
    db.commit()
    db.refresh(watch_item)

    # 2. Mock price drop
    with patch("app.services.price_lookup.lookup_price_google_shopping") as mock_lookup:
        mock_lookup.return_value = (
            45.0,  # Price dropped to $45 (below target of $50)
            "USD",
            "TestRetailer",
            "https://example.com/buy",
            {"title": "Test Product"},
            [],
        )

        # Run monitoring
        run_price_monitoring()

        # 3. Verify notifications were queued
        db.refresh(watch_item)
        assert watch_item.best_price == 45.0

        notifications = (
            db.query(NotificationQueue)
            .filter(
                NotificationQueue.watch_item_id == watch_item.id,
                NotificationQueue.is_sent == False,
            )
            .all()
        )

        assert len(notifications) >= 1  # At least target_hit notification

        # 4. Mock WhatsApp delivery
        with patch("app.services.notification_delivery.send_whatsapp_message") as mock_send:
            mock_send.return_value = True

            # Deliver notifications
            result = deliver_pending_notifications(db)
            assert result["sent"] >= 1

            # 5. Verify notifications marked as sent
            for n in notifications:
                db.refresh(n)
                assert n.is_sent is True
                assert n.sent_at is not None

    # Cleanup
    for notification in notifications:
        db.delete(notification)
    db.delete(watch_item)
    db.commit()
    db.close()
