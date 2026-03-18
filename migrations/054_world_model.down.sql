-- migrations/054_world_model.down.sql
DROP POLICY IF EXISTS wmf_isolation ON world_model_facts;
DROP TABLE IF EXISTS world_model_facts;
