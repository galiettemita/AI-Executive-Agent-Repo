DROP INDEX IF EXISTS idx_pg_security_scores_run_at;
DROP INDEX IF EXISTS idx_pg_security_scores_run_id;
DROP TABLE IF EXISTS pg_security_scores;

DROP INDEX IF EXISTS idx_red_team_attempts_created_at;
DROP INDEX IF EXISTS idx_red_team_attempts_attack_type;
DROP TABLE IF EXISTS red_team_attempts;
