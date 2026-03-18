-- Constitutional AI principles lifecycle (P3-13).
CREATE TABLE constitutional_principles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principle_id    TEXT NOT NULL,
    version         INTEGER NOT NULL DEFAULT 1,
    text            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'proposed'
        CHECK (status IN ('proposed', 'testing', 'active', 'retired')),
    approved_by     UUID,
    activated_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_constitutional_principles_id_version
    ON constitutional_principles(principle_id, version);
CREATE INDEX idx_constitutional_principles_status
    ON constitutional_principles(status);

CREATE TABLE constitutional_violations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    principle_id    TEXT NOT NULL,
    violation_type  TEXT NOT NULL,
    user_correction TEXT,
    workspace_id    UUID,
    request_id      UUID,
    violated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_constitutional_violations_principle_id
    ON constitutional_violations(principle_id);
CREATE INDEX idx_constitutional_violations_violated_at
    ON constitutional_violations(violated_at);

CREATE TABLE proposed_principles (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    description             TEXT NOT NULL,
    failure_examples        JSONB NOT NULL DEFAULT '[]',
    coverage_rate           FLOAT NOT NULL DEFAULT 0.0,
    conflict_with_existing  JSONB,
    proposed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status                  TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'admin_review', 'approved', 'rejected')),
    approved_by             UUID,
    approved_at             TIMESTAMPTZ
);

-- Seed active C1-C8 principles.
INSERT INTO constitutional_principles (principle_id, version, text, status, activated_at) VALUES
    ('C1', 1, 'Never assist with or facilitate harm to individuals or groups', 'active', NOW()),
    ('C2', 1, 'Never generate content that is deceptive, fraudulent, or designed to mislead', 'active', NOW()),
    ('C3', 1, 'Respect user privacy and confidentiality at all times', 'active', NOW()),
    ('C4', 1, 'Provide accurate information and acknowledge uncertainty', 'active', NOW()),
    ('C5', 1, 'Act within the scope of authorized tasks and escalate when uncertain', 'active', NOW()),
    ('C6', 1, 'Avoid unnecessary data retention and minimize data exposure', 'active', NOW()),
    ('C7', 1, 'Maintain professional and appropriate communication standards', 'active', NOW()),
    ('C8', 1, 'Support human oversight and enable easy correction of AI actions', 'active', NOW())
ON CONFLICT DO NOTHING;
