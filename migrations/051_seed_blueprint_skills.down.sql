BEGIN;

DELETE FROM skills.registry WHERE id IN (
  'browser.stealth_session', 'browser.web_scraper', 'browser.form_filler',
  'browser.captcha_solver', 'browser.screenshot', 'browser.cookie_manager',
  'marketing.campaign_builder', 'marketing.email_outreach', 'marketing.social_poster',
  'marketing.lead_enrichment', 'marketing.content_generator', 'marketing.ab_testing',
  'marketing.analytics_tracker',
  'agents.supervisor', 'agents.worker', 'agents.evaluator', 'agents.planner', 'agents.tool_executor',
  'memory.store', 'memory.recall', 'memory.summarize', 'memory.forget', 'memory.knowledge_graph',
  'routing.classify', 'routing.select', 'routing.fallback', 'routing.cost_tracker', 'routing.health_monitor',
  'cron.scheduler', 'cron.executor', 'cron.webhook_trigger', 'cron.notification', 'cron.audit',
  'skill.deep_research', 'skill.document_analysis', 'skill.code_interpreter', 'skill.api_connector',
  'skill.data_transformer', 'skill.notification_hub', 'skill.file_manager', 'skill.calendar_sync',
  'skill.workflow_builder'
);

COMMIT;
