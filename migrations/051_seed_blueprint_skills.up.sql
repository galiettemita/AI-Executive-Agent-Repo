BEGIN;

INSERT INTO skills.registry (id, category, plane, impact, description, brevio_use_case, min_tier, deployment_mode, cost_per_invocation_cents)
VALUES
  -- Browser & Automation (6 skills)
  ('browser.stealth_session', 'browser', 'hands', 'HIGH', 'Launch undetectable browser sessions with fingerprint rotation', 'Automate web tasks without detection', 'pro', 'cloud', 0.5),
  ('browser.web_scraper', 'browser', 'hands', 'MEDIUM', 'Extract structured data from any webpage', 'Gather competitive intelligence and market data', 'free', 'cloud', 0.2),
  ('browser.form_filler', 'browser', 'hands', 'MEDIUM', 'Automatically fill and submit web forms', 'Automate repetitive form submissions', 'pro', 'cloud', 0.3),
  ('browser.captcha_solver', 'browser', 'hands', 'HIGH', 'Solve CAPTCHAs using AI and third-party services', 'Bypass CAPTCHAs during automated workflows', 'pro', 'cloud', 1.0),
  ('browser.screenshot', 'browser', 'hands', 'LOW', 'Capture full-page or element screenshots', 'Visual documentation and monitoring', 'free', 'cloud', 0.1),
  ('browser.cookie_manager', 'browser', 'hands', 'MEDIUM', 'Manage and persist browser cookies across sessions', 'Maintain login sessions for automation', 'pro', 'cloud', 0.1),

  -- Marketing & Sales (7 skills)
  ('marketing.campaign_builder', 'marketing', 'hands', 'HIGH', 'Create and manage multi-channel marketing campaigns', 'Design and launch marketing campaigns via AI', 'pro', 'cloud', 0.5),
  ('marketing.email_outreach', 'marketing', 'hands', 'HIGH', 'Send personalized email sequences', 'Automated cold outreach and nurture sequences', 'pro', 'cloud', 0.3),
  ('marketing.social_poster', 'marketing', 'hands', 'MEDIUM', 'Publish content to social media platforms', 'Schedule and post across social channels', 'free', 'cloud', 0.2),
  ('marketing.lead_enrichment', 'marketing', 'hands', 'HIGH', 'Enrich contact data from multiple providers', 'Build complete prospect profiles', 'pro', 'cloud', 0.8),
  ('marketing.content_generator', 'marketing', 'brain', 'MEDIUM', 'Generate marketing content using AI', 'Create blog posts, ad copy, and social content', 'free', 'cloud', 0.4),
  ('marketing.ab_testing', 'marketing', 'hands', 'MEDIUM', 'Run A/B tests on marketing content', 'Optimize messaging and creative assets', 'pro', 'cloud', 0.2),
  ('marketing.analytics_tracker', 'marketing', 'hands', 'MEDIUM', 'Track and analyze marketing performance', 'Measure campaign ROI and engagement', 'free', 'cloud', 0.1),

  -- Multi-Agent Orchestration (5 skills)
  ('agents.supervisor', 'agents', 'brain', 'HIGH', 'Coordinate multiple AI agents for complex tasks', 'Break down complex tasks into agent workflows', 'enterprise', 'cloud', 1.0),
  ('agents.worker', 'agents', 'brain', 'MEDIUM', 'Execute subtasks as delegated by supervisors', 'Parallel execution of independent subtasks', 'pro', 'cloud', 0.5),
  ('agents.evaluator', 'agents', 'brain', 'MEDIUM', 'Evaluate and score agent outputs for quality', 'Ensure agent output meets quality standards', 'pro', 'cloud', 0.3),
  ('agents.planner', 'agents', 'brain', 'HIGH', 'Create execution plans for multi-step tasks', 'Strategic planning for complex workflows', 'pro', 'cloud', 0.5),
  ('agents.tool_executor', 'agents', 'hands', 'MEDIUM', 'Execute registered tools on behalf of agents', 'Bridge between agent decisions and tool execution', 'pro', 'cloud', 0.2),

  -- Memory Systems (5 skills)
  ('memory.store', 'memory', 'brain', 'HIGH', 'Store and index documents with embeddings', 'Build persistent knowledge base from conversations', 'free', 'cloud', 0.2),
  ('memory.recall', 'memory', 'brain', 'HIGH', 'Retrieve relevant memories using semantic search', 'Context-aware information retrieval', 'free', 'cloud', 0.1),
  ('memory.summarize', 'memory', 'brain', 'MEDIUM', 'Summarize conversations and extract key facts', 'Compress conversation history for context', 'free', 'cloud', 0.3),
  ('memory.forget', 'memory', 'brain', 'LOW', 'Remove specific memories or documents', 'Privacy-preserving memory management', 'free', 'cloud', 0.0),
  ('memory.knowledge_graph', 'memory', 'brain', 'HIGH', 'Build and query entity-relationship graphs', 'Connect facts and entities across conversations', 'pro', 'cloud', 0.4),

  -- Smart Model Routing (5 skills)
  ('routing.classify', 'routing', 'brain', 'HIGH', 'Classify request complexity for model selection', 'Route simple vs complex queries to appropriate models', 'free', 'cloud', 0.05),
  ('routing.select', 'routing', 'brain', 'HIGH', 'Select optimal model based on rules and context', 'Cost-optimized model selection', 'free', 'cloud', 0.02),
  ('routing.fallback', 'routing', 'brain', 'MEDIUM', 'Handle model failures with automatic fallback', 'Resilient model routing with degradation', 'free', 'cloud', 0.02),
  ('routing.cost_tracker', 'routing', 'brain', 'MEDIUM', 'Track and optimize model usage costs', 'Budget-aware model selection', 'free', 'cloud', 0.01),
  ('routing.health_monitor', 'routing', 'brain', 'MEDIUM', 'Monitor provider health and availability', 'Real-time provider status tracking', 'free', 'cloud', 0.01),

  -- Cron & Scheduled Workflows (5 skills)
  ('cron.scheduler', 'cron', 'hands', 'HIGH', 'Create and manage scheduled tasks', 'Set up recurring AI-powered workflows', 'free', 'cloud', 0.1),
  ('cron.executor', 'cron', 'hands', 'MEDIUM', 'Execute scheduled tasks with retry logic', 'Reliable execution of scheduled jobs', 'free', 'cloud', 0.1),
  ('cron.webhook_trigger', 'cron', 'hands', 'MEDIUM', 'Trigger webhooks on schedule', 'Integrate with external systems on schedule', 'pro', 'cloud', 0.1),
  ('cron.notification', 'cron', 'hands', 'LOW', 'Send notifications for cron events', 'Alert users on job completion/failure', 'free', 'cloud', 0.05),
  ('cron.audit', 'cron', 'hands', 'LOW', 'Track and audit all cron job activity', 'Complete audit trail for scheduled tasks', 'free', 'cloud', 0.01),

  -- Individual Named Skills (9 skills)
  ('skill.deep_research', 'research', 'brain', 'HIGH', 'Multi-source deep research with synthesis', 'Comprehensive research across web and knowledge base', 'pro', 'cloud', 1.5),
  ('skill.document_analysis', 'research', 'brain', 'MEDIUM', 'Analyze and extract insights from documents', 'Process PDFs, reports, and long documents', 'free', 'cloud', 0.5),
  ('skill.code_interpreter', 'developer', 'hands', 'HIGH', 'Execute code in sandboxed environments', 'Run Python, JS, SQL for data analysis', 'pro', 'cloud', 0.3),
  ('skill.api_connector', 'integration', 'hands', 'MEDIUM', 'Connect to and call external APIs', 'Universal API integration layer', 'pro', 'cloud', 0.2),
  ('skill.data_transformer', 'data', 'hands', 'MEDIUM', 'Transform data between formats and schemas', 'ETL operations for data pipelines', 'free', 'cloud', 0.1),
  ('skill.notification_hub', 'communication', 'hands', 'LOW', 'Multi-channel notification dispatch', 'Send alerts via email, SMS, push, and webhooks', 'free', 'cloud', 0.05),
  ('skill.file_manager', 'storage', 'hands', 'LOW', 'Manage files in cloud storage', 'Upload, organize, and share files', 'free', 'cloud', 0.05),
  ('skill.calendar_sync', 'productivity', 'hands', 'MEDIUM', 'Sync and manage calendar events', 'Schedule meetings and manage availability', 'pro', 'cloud', 0.1),
  ('skill.workflow_builder', 'automation', 'brain', 'HIGH', 'Visual workflow builder for custom automations', 'No-code workflow creation with AI assistance', 'pro', 'cloud', 0.5)
ON CONFLICT (id) DO UPDATE SET
  category = EXCLUDED.category,
  plane = EXCLUDED.plane,
  impact = EXCLUDED.impact,
  description = EXCLUDED.description,
  brevio_use_case = EXCLUDED.brevio_use_case,
  min_tier = EXCLUDED.min_tier,
  deployment_mode = EXCLUDED.deployment_mode,
  cost_per_invocation_cents = EXCLUDED.cost_per_invocation_cents,
  updated_at = now();

COMMIT;
