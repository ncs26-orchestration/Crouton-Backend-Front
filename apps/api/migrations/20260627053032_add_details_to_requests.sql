-- migrate:up
-- details holds the optional structured fields a request carries (amount, vendor,
-- dates, headcount, ...). Free-form JSON so any request shape works and the
-- agents can reason over the facts; the form supplies typed templates as a
-- convenience, not a constraint.
ALTER TABLE requests
    ADD COLUMN details JSONB NOT NULL DEFAULT '{}'::jsonb;

-- migrate:down
ALTER TABLE requests
    DROP COLUMN details;
