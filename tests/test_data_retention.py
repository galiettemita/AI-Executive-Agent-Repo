from datetime import datetime, timedelta

from app.core.config import settings
from app.db.database import SessionLocal
from app.db.models import Conversation, ChatMessage, NotificationQueue, User
from app.services.data_retention import purge_expired_records


def test_data_retention_purges_old_records():
    db = SessionLocal()
    try:
        user = User(id="retention_user")
        db.add(user)
        db.commit()

        convo = Conversation(user_id=user.id)
        db.add(convo)
        db.commit()

        old_time = datetime.utcnow() - timedelta(days=settings.RETENTION_CHAT_MESSAGES_DAYS + 1)

        db.add(
            ChatMessage(
                conversation_id=convo.id,
                user_id=user.id,
                role="user",
                content="old message",
                created_at=old_time,
            )
        )
        db.add(
            NotificationQueue(
                user_id=user.id,
                event_type="price_drop",
                title="Old notification",
                message="Old notification",
                is_sent=True,
                sent_at=old_time,
                created_at=old_time,
            )
        )
        db.commit()

        results = purge_expired_records(db)

        assert results["chat_messages"] >= 1
        assert results["notification_queue"] >= 1
    finally:
        db.close()
