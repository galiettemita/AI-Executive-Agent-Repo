-- Reverse migration 053: P8 Feature Closures

DROP TABLE IF EXISTS load_shedding_state CASCADE;
DROP TABLE IF EXISTS onboarding_sessions CASCADE;
DROP TABLE IF EXISTS billing_ledger_entries CASCADE;
DROP TABLE IF EXISTS billing_webhook_events CASCADE;
DROP TABLE IF EXISTS fast_path_routes CASCADE;
DROP TABLE IF EXISTS experiment_conversions CASCADE;
DROP TABLE IF EXISTS experiment_assignments CASCADE;
DROP TABLE IF EXISTS experiment_definitions CASCADE;
DROP TABLE IF EXISTS browser_sessions CASCADE;
DROP TABLE IF EXISTS edge_sync_tasks CASCADE;
DROP TABLE IF EXISTS federation_sync_log CASCADE;
