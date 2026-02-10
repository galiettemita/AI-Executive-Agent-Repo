# backend/app/db/models.py

from __future__ import annotations

from datetime import datetime
from sqlalchemy import Boolean, Date, DateTime, Float, ForeignKey, Integer, String, Text, UniqueConstraint
from sqlalchemy.orm import Mapped, mapped_column
# app/db/models.py

from app.db.database import Base


class User(Base):
    __tablename__ = "users"

    id: Mapped[str] = mapped_column(String, primary_key=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)


# -------------------
# BETA TESTERS
# -------------------

class BetaTester(Base):
    __tablename__ = "beta_testers"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, unique=True, index=True)
    email: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    status: Mapped[str] = mapped_column(String, default="active", index=True)  # active, paused, removed
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


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
    watch_item_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("watch_items.id"), index=True, nullable=True)

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
# AUDIT LOGS
# -------------------

class AuditLog(Base):
    __tablename__ = "audit_logs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str | None] = mapped_column(String, ForeignKey("users.id"), index=True, nullable=True)

    actor_type: Mapped[str] = mapped_column(String, default="user", index=True)  # user, system, webhook
    action: Mapped[str] = mapped_column(String, index=True)  # e.g., http_request

    resource_type: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    resource_id: Mapped[str | None] = mapped_column(String, nullable=True, index=True)

    method: Mapped[str | None] = mapped_column(String, nullable=True)
    path: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    status_code: Mapped[int | None] = mapped_column(Integer, nullable=True)

    ip_address: Mapped[str | None] = mapped_column(String, nullable=True)
    user_agent: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# CONTACTS & MESSAGING
# -------------------

class Contact(Base):
    __tablename__ = "contacts"
    __table_args__ = (
        UniqueConstraint("user_id", "normalized_phone", name="uq_contacts_user_phone"),
        UniqueConstraint("user_id", "normalized_email", name="uq_contacts_user_email"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    name: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    phone: Mapped[str | None] = mapped_column(String, nullable=True)
    email: Mapped[str | None] = mapped_column(String, nullable=True)
    normalized_phone: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    normalized_email: Mapped[str | None] = mapped_column(String, nullable=True, index=True)

    tags_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class OutboundMessage(Base):
    __tablename__ = "outbound_messages"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    contact_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("contacts.id"), nullable=True, index=True)

    channel: Mapped[str] = mapped_column(String, index=True)  # whatsapp, sms, email
    to_address: Mapped[str] = mapped_column(String, index=True)
    body: Mapped[str] = mapped_column(Text)

    status: Mapped[str] = mapped_column(String, default="queued", index=True)  # queued, sending, sent, failed
    provider: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    provider_status: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    provider_message_id: Mapped[str | None] = mapped_column(String, nullable=True)
    error_message: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    sent_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    delivered_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    failed_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    last_status_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)


class OutboundMessageEvent(Base):
    __tablename__ = "outbound_message_events"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    message_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("outbound_messages.id"), nullable=True, index=True)
    provider: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    event_type: Mapped[str] = mapped_column(String, index=True)
    payload_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# RELATIONSHIP MANAGER
# -------------------

class RelationshipProfile(Base):
    __tablename__ = "relationship_profiles"
    __table_args__ = (
        UniqueConstraint("user_id", "contact_id", name="uq_relationship_profile_contact"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    contact_id: Mapped[int] = mapped_column(Integer, ForeignKey("contacts.id"), index=True)

    relationship: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    priority: Mapped[int | None] = mapped_column(Integer, nullable=True)
    cadence_days: Mapped[int] = mapped_column(Integer, default=30)
    preferred_channel: Mapped[str | None] = mapped_column(String, nullable=True)

    tags_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    last_interaction_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    last_inbound_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    last_outbound_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    next_checkin_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class RelationshipInteraction(Base):
    __tablename__ = "relationship_interactions"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    contact_id: Mapped[int] = mapped_column(Integer, ForeignKey("contacts.id"), index=True)

    direction: Mapped[str] = mapped_column(String, default="outbound", index=True)  # inbound|outbound
    channel: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    summary: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    occurred_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# FITNESS & NUTRITION
# -------------------

class FitnessWorkout(Base):
    __tablename__ = "fitness_workouts"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    workout_type: Mapped[str] = mapped_column(String, index=True)
    duration_minutes: Mapped[int | None] = mapped_column(Integer, nullable=True)
    calories_burned: Mapped[float | None] = mapped_column(Float, nullable=True)
    intensity: Mapped[str | None] = mapped_column(String, nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    occurred_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class FitnessMealPlan(Base):
    __tablename__ = "fitness_meal_plans"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    plan_date: Mapped[datetime] = mapped_column(DateTime, index=True)
    meals_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    calorie_target: Mapped[int | None] = mapped_column(Integer, nullable=True)
    macros_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class NutritionLog(Base):
    __tablename__ = "nutrition_logs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    meal_type: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    calories: Mapped[int | None] = mapped_column(Integer, nullable=True)
    protein_g: Mapped[float | None] = mapped_column(Float, nullable=True)
    carbs_g: Mapped[float | None] = mapped_column(Float, nullable=True)
    fat_g: Mapped[float | None] = mapped_column(Float, nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    occurred_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class FitnessStepLog(Base):
    __tablename__ = "fitness_step_logs"
    __table_args__ = (
        UniqueConstraint("user_id", "step_date", name="uq_fitness_steps_user_date"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    step_date: Mapped[datetime] = mapped_column(Date, index=True)
    steps: Mapped[int] = mapped_column(Integer, default=0)
    source: Mapped[str] = mapped_column(String, default="fitbit", index=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


# -------------------
# ENTERTAINMENT
# -------------------

class EntertainmentItem(Base):
    __tablename__ = "entertainment_items"
    __table_args__ = (
        UniqueConstraint("user_id", "external_url", name="uq_entertainment_user_url"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    title: Mapped[str] = mapped_column(String, index=True)
    content_type: Mapped[str] = mapped_column(String, index=True)
    status: Mapped[str] = mapped_column(String, default="planned", index=True)
    rating: Mapped[float | None] = mapped_column(Float, nullable=True)

    external_url: Mapped[str | None] = mapped_column(String, nullable=True)
    source: Mapped[str | None] = mapped_column(String, nullable=True, index=True)

    tags_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    last_consumed_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class EntertainmentConsumption(Base):
    __tablename__ = "entertainment_consumption"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    item_id: Mapped[int] = mapped_column(Integer, ForeignKey("entertainment_items.id"), index=True)

    event_type: Mapped[str] = mapped_column(String, default="watched", index=True)
    duration_minutes: Mapped[int | None] = mapped_column(Integer, nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    occurred_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# PHONE VERIFICATION
# -------------------

class PhoneVerification(Base):
    __tablename__ = "phone_verifications"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    phone_number: Mapped[str] = mapped_column(String, index=True)

    code_hash: Mapped[str] = mapped_column(String, nullable=False)
    status: Mapped[str] = mapped_column(String, default="pending", index=True)  # pending, verified, expired, locked

    attempts: Mapped[int] = mapped_column(Integer, default=0)
    max_attempts: Mapped[int] = mapped_column(Integer, default=5)

    expires_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    verified_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    last_sent_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


# -------------------
# PROACTIVE RULES
# -------------------

class ProactiveRule(Base):
    __tablename__ = "proactive_rules"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    name: Mapped[str] = mapped_column(String, index=True)
    is_active: Mapped[bool] = mapped_column(Boolean, default=True)

    trigger_type: Mapped[str] = mapped_column(String, index=True)  # interval, daily, once
    trigger_config_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    conditions_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    action_type: Mapped[str] = mapped_column(String, index=True)  # notify, create_proposal, voice_call_proposal
    action_payload_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    last_run_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    next_run_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class ProactiveRuleRun(Base):
    __tablename__ = "proactive_rule_runs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    rule_id: Mapped[int] = mapped_column(Integer, ForeignKey("proactive_rules.id"), index=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    status: Mapped[str] = mapped_column(String, index=True)  # ok, skipped, failed
    reason: Mapped[str | None] = mapped_column(Text, nullable=True)

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


class IntegrationCredential(Base):
    __tablename__ = "integration_credentials"
    __table_args__ = (
        UniqueConstraint("user_id", "provider", name="uq_integration_credentials_user_provider"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    # e.g. "caldav"
    provider: Mapped[str] = mapped_column(String, index=True)

    username: Mapped[str | None] = mapped_column(String, nullable=True)
    secret_enc: Mapped[str | None] = mapped_column(Text, nullable=True)
    server_url: Mapped[str | None] = mapped_column(String, nullable=True)

    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class EmailDraft(Base):
    __tablename__ = "email_drafts"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    provider: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    to_email: Mapped[str] = mapped_column(String, index=True)
    cc: Mapped[str | None] = mapped_column(String, nullable=True)
    bcc: Mapped[str | None] = mapped_column(String, nullable=True)
    subject: Mapped[str] = mapped_column(String)
    body_text: Mapped[str] = mapped_column(Text)

    source_message_id: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    provider_draft_id: Mapped[str | None] = mapped_column(String, nullable=True)
    status: Mapped[str] = mapped_column(String, default="pending", index=True)  # pending, sent, canceled, failed

    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)
    sent_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)


class EmailMonitorConfig(Base):
    __tablename__ = "email_monitor_configs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    provider: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    enabled: Mapped[bool] = mapped_column(Boolean, default=True, index=True)

    keywords_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    sender_allowlist_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    subject_keywords_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    priority_threshold: Mapped[int | None] = mapped_column(Integer, nullable=True)
    use_ai_priority: Mapped[bool] = mapped_column(Boolean, default=False)

    alert_channel: Mapped[str] = mapped_column(String, default="whatsapp", index=True)
    alert_title: Mapped[str | None] = mapped_column(String, nullable=True)

    window_minutes: Mapped[int] = mapped_column(Integer, default=60)
    max_results: Mapped[int] = mapped_column(Integer, default=20)

    last_checked_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class EmailAlert(Base):
    __tablename__ = "email_alerts"
    __table_args__ = (
        UniqueConstraint(
            "user_id",
            "provider",
            "message_id",
            "alert_channel",
            name="uq_email_alert_user_message",
        ),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    provider: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    message_id: Mapped[str] = mapped_column(String, index=True)
    subject: Mapped[str | None] = mapped_column(String, nullable=True)
    sender: Mapped[str | None] = mapped_column(String, nullable=True)
    priority: Mapped[int | None] = mapped_column(Integer, nullable=True)
    reason: Mapped[str | None] = mapped_column(Text, nullable=True)

    alert_channel: Mapped[str] = mapped_column(String, default="whatsapp", index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


class TaskItem(Base):
    __tablename__ = "tasks"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    title: Mapped[str] = mapped_column(String, index=True)
    due_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)

    completed: Mapped[bool] = mapped_column(Boolean, default=False, index=True)
    completed_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


class FileAsset(Base):
    __tablename__ = "file_assets"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    filename: Mapped[str] = mapped_column(String, index=True)
    content_type: Mapped[str | None] = mapped_column(String, nullable=True)
    size_bytes: Mapped[int | None] = mapped_column(Integer, nullable=True)
    storage_key: Mapped[str] = mapped_column(String, unique=True, index=True)

    tags_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class PhotoAsset(Base):
    __tablename__ = "photo_assets"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    filename: Mapped[str] = mapped_column(String, index=True)
    content_type: Mapped[str | None] = mapped_column(String, nullable=True)
    size_bytes: Mapped[int | None] = mapped_column(Integer, nullable=True)
    storage_key: Mapped[str] = mapped_column(String, unique=True, index=True)

    tags_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


# -------------------
# WARDROBE
# -------------------

class WardrobeItem(Base):
    __tablename__ = "wardrobe_items"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    name: Mapped[str] = mapped_column(String, index=True)
    category: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    subcategory: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    brand: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    color: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    size: Mapped[str | None] = mapped_column(String, nullable=True)
    material: Mapped[str | None] = mapped_column(String, nullable=True)
    season: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    condition: Mapped[str | None] = mapped_column(String, nullable=True)

    purchase_date: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    price: Mapped[float | None] = mapped_column(Float, nullable=True)
    currency: Mapped[str | None] = mapped_column(String, nullable=True)

    notes: Mapped[str | None] = mapped_column(Text, nullable=True)
    tags_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    wear_count: Mapped[int] = mapped_column(Integer, default=0, index=True)
    last_worn_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)
    last_rotation_notified_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class WardrobeItemPhoto(Base):
    __tablename__ = "wardrobe_item_photos"
    __table_args__ = (
        UniqueConstraint("wardrobe_item_id", "photo_asset_id", name="uq_wardrobe_item_photo"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    wardrobe_item_id: Mapped[int] = mapped_column(Integer, ForeignKey("wardrobe_items.id"), index=True)
    photo_asset_id: Mapped[int] = mapped_column(Integer, ForeignKey("photo_assets.id"), index=True)

    is_primary: Mapped[bool] = mapped_column(Boolean, default=False, index=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


class WardrobeWearEvent(Base):
    __tablename__ = "wardrobe_wear_events"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    wardrobe_item_id: Mapped[int] = mapped_column(Integer, ForeignKey("wardrobe_items.id"), index=True)

    worn_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    source: Mapped[str] = mapped_column(String, default="manual", index=True)  # manual, auto
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


class GiftOccasion(Base):
    __tablename__ = "gift_occasions"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    recipient_name: Mapped[str] = mapped_column(String, index=True)
    relationship: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    occasion_type: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    occasion_date: Mapped[datetime | None] = mapped_column(Date, nullable=True, index=True)
    recurrence: Mapped[str | None] = mapped_column(String, nullable=True, index=True)  # annual, none

    reminder_days_before: Mapped[int] = mapped_column(Integer, default=14)
    last_reminder_sent_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True, index=True)

    budget: Mapped[float | None] = mapped_column(Float, nullable=True)
    currency: Mapped[str | None] = mapped_column(String, nullable=True)

    preferences_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    notes: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class GiftIdea(Base):
    __tablename__ = "gift_ideas"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    occasion_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("gift_occasions.id"), nullable=True, index=True)

    title: Mapped[str] = mapped_column(String, index=True)
    description: Mapped[str | None] = mapped_column(Text, nullable=True)
    link_url: Mapped[str | None] = mapped_column(String, nullable=True)
    price: Mapped[float | None] = mapped_column(Float, nullable=True)
    currency: Mapped[str | None] = mapped_column(String, nullable=True)
    status: Mapped[str] = mapped_column(String, default="idea", index=True)  # idea, shortlisted, purchased, gifted
    source: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    tags_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class GiftThankYouDraft(Base):
    __tablename__ = "gift_thank_you_drafts"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    occasion_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("gift_occasions.id"), nullable=True, index=True)
    gift_idea_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("gift_ideas.id"), nullable=True, index=True)

    message: Mapped[str] = mapped_column(Text)
    status: Mapped[str] = mapped_column(String, default="draft", index=True)  # draft, sent
    sent_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class UserProfile(Base):
    __tablename__ = "user_profiles"

    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), primary_key=True)
    data_json: Mapped[str] = mapped_column(Text, default="{}")
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class UserConsent(Base):
    __tablename__ = "user_consents"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    integration: Mapped[str] = mapped_column(String, index=True)  # e.g. calendar, email, voice, files
    granted_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    revoked_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)


# -------------------
# STAGE 1: SMART HOME
# -------------------

class SmartHomeDevice(Base):
    __tablename__ = "smart_home_devices"
    __table_args__ = (
        UniqueConstraint("user_id", "provider", "provider_device_id", name="uq_smart_home_device_provider"),
    )

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    provider: Mapped[str] = mapped_column(String, index=True)  # home_assistant, google, alexa, homekit
    provider_device_id: Mapped[str] = mapped_column(String, index=True)

    name: Mapped[str] = mapped_column(String, index=True)
    device_type: Mapped[str | None] = mapped_column(String, nullable=True)  # light, switch, climate, etc.
    room: Mapped[str | None] = mapped_column(String, nullable=True)

    traits_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    state_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    online: Mapped[bool] = mapped_column(Boolean, default=True)

    last_state_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    last_seen_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class SmartHomeScene(Base):
    __tablename__ = "smart_home_scenes"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    name: Mapped[str] = mapped_column(String, index=True)
    description: Mapped[str | None] = mapped_column(String, nullable=True)
    actions_json: Mapped[str] = mapped_column(Text, default="[]")

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class SmartHomeActionLog(Base):
    __tablename__ = "smart_home_action_logs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    device_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("smart_home_devices.id"), nullable=True)
    scene_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("smart_home_scenes.id"), nullable=True)

    action_type: Mapped[str] = mapped_column(String, index=True)  # execute, scene
    status: Mapped[str] = mapped_column(String, index=True)  # ok, failed
    request_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    response_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    error_message: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


class SmartHomeEnergyAlert(Base):
    __tablename__ = "smart_home_energy_alerts"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    provider: Mapped[str] = mapped_column(String, index=True)
    entity_id: Mapped[str] = mapped_column(String, index=True)

    comparison: Mapped[str] = mapped_column(String, default="gt")  # gt, lt
    threshold_value: Mapped[float] = mapped_column(Float, default=0.0)
    unit: Mapped[str | None] = mapped_column(String, nullable=True)

    last_triggered_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


class SmartHomeEnergyReading(Base):
    __tablename__ = "smart_home_energy_readings"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    provider: Mapped[str] = mapped_column(String, index=True)
    entity_id: Mapped[str] = mapped_column(String, index=True)

    reading_time: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    value: Mapped[float] = mapped_column(Float, default=0.0)
    unit: Mapped[str | None] = mapped_column(String, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

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


class UsageEvent(Base):
    __tablename__ = "usage_events"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    event_type: Mapped[str] = mapped_column(String, index=True)
    source: Mapped[str | None] = mapped_column(String, nullable=True)
    channel: Mapped[str | None] = mapped_column(String, nullable=True)
    provider: Mapped[str | None] = mapped_column(String, nullable=True)

    tokens: Mapped[int | None] = mapped_column(Integer, nullable=True)
    cost_usd: Mapped[float | None] = mapped_column(Float, nullable=True)

    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# STAGE 4: PROPOSALS
# -------------------

class Proposal(Base):
    __tablename__ = "proposals"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    proposal_type: Mapped[str] = mapped_column(String, index=True)
    status: Mapped[str] = mapped_column(String, default="pending", index=True)
    payload_json: Mapped[str] = mapped_column(Text, default="{}")

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    expires_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)


class ProposalAuditLog(Base):
    __tablename__ = "proposal_audit_logs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    proposal_id: Mapped[int] = mapped_column(Integer, ForeignKey("proposals.id"), index=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    action: Mapped[str] = mapped_column(String, index=True)  # "created", "approved", "canceled", "edited"
    old_status: Mapped[str | None] = mapped_column(String, nullable=True)
    new_status: Mapped[str | None] = mapped_column(String, nullable=True)

    # Store changes for edit actions (JSON)
    changes_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Optional metadata (IP, user agent, etc.)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


# -------------------
# EXECUTION ENGINE - STAGE 10-16
# -------------------

class PaymentMethod(Base):
    """Stores user payment methods (Stripe payment method IDs)"""
    __tablename__ = "payment_methods"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    # Stripe payment method ID
    stripe_payment_method_id: Mapped[str] = mapped_column(String, unique=True, index=True)

    # Payment method type (card, bank_account, etc.)
    type: Mapped[str] = mapped_column(String, default="card")  # card, bank_account, etc.

    # Card details (for display only, not for processing)
    brand: Mapped[str | None] = mapped_column(String, nullable=True)  # visa, mastercard, amex, etc.
    last4: Mapped[str | None] = mapped_column(String, nullable=True)
    exp_month: Mapped[int | None] = mapped_column(Integer, nullable=True)
    exp_year: Mapped[int | None] = mapped_column(Integer, nullable=True)

    # Default payment method flag
    is_default: Mapped[bool] = mapped_column(Boolean, default=False)

    # Metadata
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class Transaction(Base):
    """Records all financial transactions"""
    __tablename__ = "transactions"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    proposal_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("proposals.id"), nullable=True, index=True)

    # Transaction details
    amount: Mapped[float] = mapped_column(Float)  # in currency's smallest unit (e.g., cents)
    currency: Mapped[str] = mapped_column(String, default="USD")
    status: Mapped[str] = mapped_column(String, default="pending", index=True)  # pending, succeeded, failed, refunded, canceled

    # Stripe payment intent ID
    stripe_payment_intent_id: Mapped[str | None] = mapped_column(String, unique=True, nullable=True, index=True)
    stripe_charge_id: Mapped[str | None] = mapped_column(String, nullable=True)

    # Payment method used
    payment_method_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("payment_methods.id"), nullable=True)

    # Transaction type and metadata
    transaction_type: Mapped[str] = mapped_column(String, index=True)  # food_order, flight_booking, hotel_booking, retail_purchase
    description: Mapped[str | None] = mapped_column(Text, nullable=True)
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)  # Store additional data as JSON

    # Refund tracking
    refund_amount: Mapped[float | None] = mapped_column(Float, nullable=True)
    refund_reason: Mapped[str | None] = mapped_column(String, nullable=True)
    refunded_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    # Invoice PDF
    invoice_pdf_path: Mapped[str | None] = mapped_column(String, nullable=True)  # Path to generated PDF invoice

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class SpendingLimit(Base):
    """Enforces spending caps per user/plan"""
    __tablename__ = "spending_limits"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    # Limit configuration
    period: Mapped[str] = mapped_column(String, index=True)  # daily, weekly, monthly
    limit_amount: Mapped[float] = mapped_column(Float)  # Max spend in this period
    spent_amount: Mapped[float] = mapped_column(Float, default=0.0)  # Current spend

    # Period tracking
    period_start: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow)
    period_end: Mapped[datetime] = mapped_column(DateTime)

    # Transaction limits
    max_transaction_amount: Mapped[float | None] = mapped_column(Float, nullable=True)  # Max per transaction
    max_transactions_per_hour: Mapped[int | None] = mapped_column(Integer, nullable=True)

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class ExecutionLog(Base):
    """Tracks execution steps for debugging and audit"""
    __tablename__ = "execution_logs"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    proposal_id: Mapped[int] = mapped_column(Integer, ForeignKey("proposals.id"), index=True)
    transaction_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("transactions.id"), nullable=True, index=True)

    # Execution step details
    step: Mapped[str] = mapped_column(String, index=True)  # validate_approval, create_payment_intent, execute_booking, send_confirmation
    status: Mapped[str] = mapped_column(String, index=True)  # started, completed, failed, retrying

    # Error tracking
    error_message: Mapped[str | None] = mapped_column(Text, nullable=True)
    error_code: Mapped[str | None] = mapped_column(String, nullable=True)
    stack_trace: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Retry tracking
    retry_count: Mapped[int] = mapped_column(Integer, default=0)

    # Metadata
    metadata_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)


class Booking(Base):
    """Stores booking details for flights, hotels, food orders, retail purchases"""
    __tablename__ = "bookings"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    proposal_id: Mapped[int] = mapped_column(Integer, ForeignKey("proposals.id"), index=True)
    transaction_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("transactions.id"), nullable=True, index=True)

    # Booking type and provider
    booking_type: Mapped[str] = mapped_column(String, index=True)  # flight, hotel, food_order, retail_purchase
    provider: Mapped[str] = mapped_column(String, index=True)  # amadeus, doordash, amazon, walmart, etc.

    # Booking status
    status: Mapped[str] = mapped_column(String, default="pending", index=True)  # pending, confirmed, in_progress, completed, canceled, failed

    # Confirmation details
    confirmation_number: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    provider_booking_id: Mapped[str | None] = mapped_column(String, nullable=True, index=True)

    # PNR for travel bookings
    pnr: Mapped[str | None] = mapped_column(String, nullable=True, index=True)

    # Booking payload (full details as JSON)
    payload_json: Mapped[str] = mapped_column(Text, default="{}")

    # Cancellation tracking
    canceled_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    cancellation_reason: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


# -------------------
# STAGE 14: WEBHOOKS
# -------------------

class WebhookEndpoint(Base):
    """Stores user webhook endpoints for execution status notifications"""
    __tablename__ = "webhook_endpoints"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    # Webhook configuration
    url: Mapped[str] = mapped_column(String)  # Webhook URL
    secret: Mapped[str | None] = mapped_column(String, nullable=True)  # Optional secret for signature verification
    is_active: Mapped[bool] = mapped_column(Boolean, default=True, index=True)

    # Event filters (JSON list of event types to subscribe to)
    # e.g., ["proposal.approved", "execution.started", "execution.completed", "execution.failed"]
    event_types: Mapped[str | None] = mapped_column(Text, nullable=True)  # JSON array

    # Metadata
    description: Mapped[str | None] = mapped_column(String, nullable=True)

    # Statistics
    total_deliveries: Mapped[int] = mapped_column(Integer, default=0)
    failed_deliveries: Mapped[int] = mapped_column(Integer, default=0)
    last_delivery_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    last_failure_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


class WebhookDelivery(Base):
    """Logs webhook delivery attempts for debugging and monitoring"""
    __tablename__ = "webhook_deliveries"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    webhook_endpoint_id: Mapped[int] = mapped_column(Integer, ForeignKey("webhook_endpoints.id"), index=True)

    # Event details
    event_type: Mapped[str] = mapped_column(String, index=True)  # e.g., "execution.started"
    event_id: Mapped[str] = mapped_column(String, index=True)  # Unique identifier for idempotency

    # Related entities
    proposal_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("proposals.id"), nullable=True, index=True)
    transaction_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("transactions.id"), nullable=True, index=True)

    # Delivery details
    payload_json: Mapped[str] = mapped_column(Text)  # Full webhook payload
    status: Mapped[str] = mapped_column(String, default="pending", index=True)  # pending, delivered, failed

    # Response tracking
    response_status_code: Mapped[int | None] = mapped_column(Integer, nullable=True)
    response_body: Mapped[str | None] = mapped_column(Text, nullable=True)
    error_message: Mapped[str | None] = mapped_column(Text, nullable=True)

    # Retry tracking
    retry_count: Mapped[int] = mapped_column(Integer, default=0)
    next_retry_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    delivered_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)


# -------------------
# STAGE 11: TRAVEL PROFILES
# -------------------

class TravelerProfile(Base):
    """Stores traveler information for flight and hotel bookings"""
    __tablename__ = "traveler_profiles"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    # Personal information
    first_name: Mapped[str] = mapped_column(String)
    last_name: Mapped[str] = mapped_column(String)
    middle_name: Mapped[str | None] = mapped_column(String, nullable=True)
    date_of_birth: Mapped[str] = mapped_column(String)  # YYYY-MM-DD
    gender: Mapped[str] = mapped_column(String)  # MALE, FEMALE, OTHER

    # Contact information
    email: Mapped[str | None] = mapped_column(String, nullable=True)
    phone: Mapped[str | None] = mapped_column(String, nullable=True)

    # Passport/ID information
    passport_number: Mapped[str | None] = mapped_column(String, nullable=True)
    passport_country: Mapped[str | None] = mapped_column(String, nullable=True)
    passport_expiry: Mapped[str | None] = mapped_column(String, nullable=True)  # YYYY-MM-DD
    nationality: Mapped[str | None] = mapped_column(String, nullable=True)

    # Travel preferences
    known_traveler_number: Mapped[str | None] = mapped_column(String, nullable=True)  # TSA PreCheck
    redress_number: Mapped[str | None] = mapped_column(String, nullable=True)
    seat_preference: Mapped[str | None] = mapped_column(String, nullable=True)  # WINDOW, AISLE, MIDDLE
    meal_preference: Mapped[str | None] = mapped_column(String, nullable=True)  # VEGAN, VEGETARIAN, etc.

    # Frequent flyer programs (JSON)
    loyalty_programs: Mapped[str | None] = mapped_column(Text, nullable=True)  # JSON array

    # Default profile flag
    is_default: Mapped[bool] = mapped_column(Boolean, default=False)

    # Timestamps
    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)


# -------------------
# STAGE 18: VOICE CALLS
# -------------------

class VoiceCallScript(Base):
    __tablename__ = "voice_call_scripts"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)

    name: Mapped[str] = mapped_column(String, index=True)
    description: Mapped[str | None] = mapped_column(Text, nullable=True)
    script_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow, index=True)


class VoiceCall(Base):
    """Stores voice call details for Twilio Voice"""
    __tablename__ = "voice_calls"

    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    user_id: Mapped[str] = mapped_column(String, ForeignKey("users.id"), index=True)
    proposal_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("proposals.id"), nullable=True, index=True)
    script_id: Mapped[int | None] = mapped_column(Integer, ForeignKey("voice_call_scripts.id"), nullable=True, index=True)

    direction: Mapped[str] = mapped_column(String, index=True)  # inbound, outbound
    to_number: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    from_number: Mapped[str | None] = mapped_column(String, nullable=True, index=True)

    purpose: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    voice_profile: Mapped[str | None] = mapped_column(String, nullable=True)
    status: Mapped[str] = mapped_column(String, default="initiating", index=True)
    script_json: Mapped[str | None] = mapped_column(Text, nullable=True)

    duration_seconds: Mapped[int | None] = mapped_column(Integer, nullable=True)
    recording_url: Mapped[str | None] = mapped_column(String, nullable=True)
    answered_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)
    ended_at: Mapped[datetime | None] = mapped_column(DateTime, nullable=True)

    transcript: Mapped[str | None] = mapped_column(Text, nullable=True)
    summary: Mapped[str | None] = mapped_column(Text, nullable=True)
    action_items_json: Mapped[str | None] = mapped_column(Text, nullable=True)
    outcome_status: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    outcome_notes: Mapped[str | None] = mapped_column(Text, nullable=True)
    error_message: Mapped[str | None] = mapped_column(Text, nullable=True)

    call_sid: Mapped[str | None] = mapped_column(String, nullable=True, index=True)
    stream_sid: Mapped[str | None] = mapped_column(String, nullable=True, index=True)

    created_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, index=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime, default=datetime.utcnow, onupdate=datetime.utcnow)
