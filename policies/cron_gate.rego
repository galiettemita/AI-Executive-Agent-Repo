package brevio.cron_gate

default allow = true

deny[msg] {
  input.active_jobs_count > 50
  input.user_tier == "free"
  msg := "CRON_FREE_TIER_JOB_LIMIT"
}

deny[msg] {
  input.active_jobs_count > 500
  input.user_tier == "pro"
  msg := "CRON_PRO_TIER_JOB_LIMIT"
}

deny[msg] {
  input.min_interval_seconds < 60
  input.user_tier == "free"
  msg := "CRON_FREE_TIER_MIN_INTERVAL"
}

deny[msg] {
  input.webhook_url != ""
  input.user_tier == "free"
  msg := "CRON_WEBHOOK_PRO_ONLY"
}
