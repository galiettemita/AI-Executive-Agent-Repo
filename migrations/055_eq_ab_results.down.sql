-- migrations/055_eq_ab_results.down.sql
DROP POLICY IF EXISTS eq_ab_isolation ON eq_ab_results;
DROP TABLE IF EXISTS eq_ab_results;
