package brevio.marketing_gate

default allow = false

allow {
  input.user_tier != "free"
  input.skill_id == "marketing.campaign_builder"
}

allow {
  input.user_tier != "free"
  input.skill_id == "marketing.email_outreach"
}

allow {
  input.skill_id == "marketing.social_poster"
}

allow {
  input.user_tier != "free"
  input.skill_id == "marketing.lead_enrichment"
}

allow {
  input.skill_id == "marketing.content_generator"
}

allow {
  input.user_tier != "free"
  input.skill_id == "marketing.ab_testing"
}

allow {
  input.skill_id == "marketing.analytics_tracker"
}

deny[msg] {
  input.skill_id == "marketing.email_outreach"
  input.daily_email_count > 500
  msg := "MARKETING_DAILY_EMAIL_LIMIT"
}

deny[msg] {
  input.skill_id == "marketing.lead_enrichment"
  input.daily_enrichment_spend_cents > 1000
  msg := "MARKETING_ENRICHMENT_BUDGET_EXCEEDED"
}

deny[msg] {
  input.skill_id == "marketing.social_poster"
  input.posts_per_hour > 10
  msg := "MARKETING_SOCIAL_RATE_LIMIT"
}
