package brevio.browser_gate

default allow = false

allow {
  input.user_tier != "free"
  input.skill_id == "browser.stealth_session"
}

allow {
  input.skill_id == "browser.web_scraper"
}

allow {
  input.skill_id == "browser.screenshot"
}

allow {
  input.user_tier != "free"
  input.skill_id == "browser.form_filler"
}

allow {
  input.user_tier != "free"
  input.skill_id == "browser.captcha_solver"
}

allow {
  input.user_tier != "free"
  input.skill_id == "browser.cookie_manager"
}

deny[msg] {
  input.skill_id == "browser.stealth_session"
  input.concurrent_sessions >= 5
  msg := "BROWSER_MAX_CONCURRENT_SESSIONS"
}

deny[msg] {
  input.skill_id == "browser.web_scraper"
  input.rate_per_minute > 60
  msg := "BROWSER_SCRAPER_RATE_LIMIT"
}

deny[msg] {
  input.skill_id == "browser.captcha_solver"
  input.daily_captcha_spend_cents > 500
  msg := "BROWSER_CAPTCHA_BUDGET_EXCEEDED"
}
