-- migrate:up

-- Telemetry-triggered incidents have no human reporter, so reported_by
-- must be nullable. The FK is kept but NULLs are naturally ignored by FK
-- constraints per SQL standard.
ALTER TABLE incidents ALTER COLUMN reported_by DROP NOT NULL;

-- migrate:down

-- Restore NOT NULL. First NULLify any rows that slipped in without a reporter.
UPDATE incidents SET reported_by = 0 WHERE reported_by IS NULL;
ALTER TABLE incidents ALTER COLUMN reported_by SET NOT NULL;
