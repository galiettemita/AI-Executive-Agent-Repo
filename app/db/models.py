# backend/app/db/models.py

from __future__ import annotations

from datetime import datetime
from sqlalchemy import Boolean, DateTime, Float, ForeignKey, Integer, String, Text
from sqlalchemy.orm import Mapped, mapped_column
# app/db/models.py

from app.db.database import Base


class User(Base):
    __tablename__ = "users"

    id: Mapped[str] = mapped_column(String, primary_key=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)


class DeviceToken(Base):
    __tablename__ = "device_tokens"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    token: Mapped[str] = mapped_column(String, unique=True, index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)


class MemoryNote(Base):
    __tablename__ = "memory_notes"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True, unique=True)
    summary: Mapped[str] = mapped_column(Text, default="")
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class UserPreference(Base):
    __tablename__ = "preferences"

    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), primary_key=True)
    data_json: Mapped[str] = mapped_column(Text, default="{}")
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)

class Conversation(Base):
    __tablename__ = "conversations"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
class ChatMessage(Base):
    __tablename__ = "chat_messages"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    conversation_id: Mapped[int] = mapped_column(Integer, index=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    role: Mapped[str] = mapped_column(String)  # "user" or "assistant"
    content: Mapped[str] = mapped_column(Text)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# ------------------------------------------------------------
# WATCHLIST MODELS (required by app/api/routes/watch_refresh.py)
# ------------------------------------------------------------

class WatchItem(Base):
    __tablename__ = "watch_items"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    # Original input
    url: Mapped[str] = mapped_column(String, index=True)
    title_hint: Mapped[str | None] = mapped_column(String, nullable=True)

    # User target
    desired_price: Mapped[float | None] = mapped_column(Float, nullable=True)

    # Latest check timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    last_checked_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    # Price tracking used by watch_refresh.py
    last_seen_price: Mapped[float | None] = mapped_column(Float, nullable=True)
    best_price: Mapped[float | None] = mapped_column(Float, nullable=True)

    currency: Mapped[str | None] = mapped_column(String, nullable=True)
    best_retailer: Mapped[str | None] = mapped_column(String, nullable=True)
    best_offer_url: Mapped[str | None] = mapped_column(String, nullable=True)

    # Product metadata (optional)
    product_key: Mapped[str | None] = mapped_column(String, nullable=True)
    best_title: Mapped[str | None] = mapped_column(String, nullable=True)
    best_description: Mapped[str | None] = mapped_column(Text, nullable=True)
    best_rating: Mapped[float | None] = mapped_column(Float, nullable=True)
    best_reviews_count: Mapped[int | None] = mapped_column(Integer, nullable=True)
    best_condition: Mapped[str | None] = mapped_column(String, nullable=True)
    best_seller_type: Mapped[str | None] = mapped_column(String, nullable=True)


class WatchOffer(Base):
    __tablename__ = "watch_offers"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, index=True)
    watch_item_id: Mapped[int] = mapped_column(Integer, ForeignKey("watch_items.id"), index=True)

    fetched_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)

    price: Mapped[float | None] = mapped_column(Float, nullable=True)
    currency: Mapped[str] = mapped_column(String, default="USD")

    retailer: Mapped[str | None] = mapped_column(String, nullable=True)
    offer_url: Mapped[str | None] = mapped_column(String, nullable=True)

    product_key: Mapped[str | None] = mapped_column(String, nullable=True)
    title: Mapped[str | None] = mapped_column(String, nullable=True)
    description: Mapped[str | None] = mapped_column(Text, nullable=True)
    rating: Mapped[float | None] = mapped_column(Float, nullable=True)
    reviews_count: Mapped[int | None] = mapped_column(Integer, nullable=True)
    condition: Mapped[str | None] = mapped_column(String, nullable=True)
    seller_type: Mapped[str | None] = mapped_column(String, nullable=True)


class NotificationQueue(Base):
    __tablename__ = "notification_queue"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)

    user_id: Mapped[str] = mapped_column(String, index=True)
    watch_item_id: Mapped[int] = mapped_column(Integer, ForeignKey("watch_items.id"), index=True)

    event_type: Mapped[str] = mapped_column(String, index=True)  # "price_drop", "target_hit"
    title: Mapped[str] = mapped_column(String)
    message: Mapped[str] = mapped_column(Text)

    deep_link_url: Mapped[str | None] = mapped_column(String, nullable=True)

    prev_price: Mapped[float | None] = mapped_column(Float, nullable=True)
    new_price: Mapped[float | None] = mapped_column(Float, nullable=True)
    currency: Mapped[str | None] = mapped_column(String, nullable=True)

    is_sent: Mapped[bool] = mapped_column(Boolean, default=False)
    sent_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# WHATSAPP IDEMPOTENCY
# -------------------

class InboundEvent(Base):
    __tablename__ = "inbound_events"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    channel: Mapped[str] = mapped_column(String, index=True)  # "whatsapp"
    external_id: Mapped[str] = mapped_column(String, index=True, unique=True)
    user_id: Mapped[str] = mapped_column(String, index=True)
    processed: Mapped[bool] = mapped_column(Boolean, default=False)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)

# -------------------
# STAGE 5: ADMIN TOOLS
# -------------------

class OAuthToken(Base):
    __tablename__ = "oauth_tokens"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    # e.g. "google"
    provider: Mapped[str] = mapped_column(String, index=True)

    # space-delimited scopes
    scopes: Mapped[str | None] = mapped_column(Text, nullable=True)

    access_token: Mapped[str] = mapped_column(Text, default="")
    refresh_token_enc: Mapped[str] = mapped_column(Text, default="")

    expiry_utc: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    # optional convenience field
    email: Mapped[str | None] = mapped_column(String, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class TaskItem(Base):
    __tablename__ = "tasks"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    title: Mapped[str] = mapped_column(String, index=True)
    due_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)

    completed: Mapped[bool] = mapped_column(Boolean, default=False, index=True)
    completed_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# STAGE 3: BILLING
# -------------------

class Subscription(Base):
    __tablename__ = "subscriptions"

    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), primary_key=True)

    plan: Mapped[str] = mapped_column(String, default="free")  # free/starter/plus/pro
    status: Mapped[str] = mapped_column(String, default="active")  # active/past_due/canceled/trialing

    provider: Mapped[str | None] = mapped_column(String, nullable=True)  # e.g. "stripe"
    provider_customer_id: Mapped[str | None] = mapped_column(String, nullable=True)
    provider_subscription_id: Mapped[str | None] = mapped_column(String, nullable=True)

    current_period_end: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class Usage(Base):
    __tablename__ = "usage"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    period: Mapped[str] = mapped_column(String, index=True)  # YYYY-MM
    messages_count: Mapped[int] = mapped_column(Integer, default=0)
    tokens_count: Mapped[int] = mapped_column(Integer, default=0)
    proposals_count: Mapped[int] = mapped_column(Integer, default=0)

    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
