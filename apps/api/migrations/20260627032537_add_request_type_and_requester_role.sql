-- migrate:up
-- request_type is the intake agent's classification of the request (hiring,
-- procurement, policy_change, budget, infra, ...); requester_role records the
-- org role of whoever submitted it, both for traceability.
ALTER TABLE requests
    ADD COLUMN request_type    TEXT NOT NULL DEFAULT 'general',
    ADD COLUMN requester_role  TEXT NOT NULL DEFAULT '';

-- migrate:down
ALTER TABLE requests
    DROP COLUMN request_type,
    DROP COLUMN requester_role;
