-- Demo data for wail@acme.me (hello org)
-- Run: docker compose exec -T postgres psql -U app -d app < scripts/seed_demo.sql

-- ── Request: "Fix vibration on Compressor A" ──────────────────────────
INSERT INTO requests (id, org_id, title, description, requester_user_id, priority, status, progress, created_at)
VALUES ('req_demo_001', 'org_cd9680ee600f6fc2',
        'Fix vibration on Compressor A',
        'Compressor A has abnormal vibration levels. Needs inspection and repair.',
        17, 'high', 'in_progress', 35, now() - interval '3 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO requests (id, org_id, title, description, requester_user_id, priority, status, progress, created_at)
VALUES ('req_demo_002', 'org_cd9680ee600f6fc2',
        'Replace pump seal on Unit B',
        'Pump B seal is leaking. Parts ordered, needs replacement.',
        17, 'medium', 'submitted', 0, now() - interval '1 hour')
ON CONFLICT (id) DO NOTHING;

INSERT INTO requests (id, org_id, title, description, requester_user_id, priority, status, progress, created_at)
VALUES ('req_demo_003', 'org_cd9680ee600f6fc2',
        'Schedule quarterly maintenance',
        'All machines need quarterly preventive maintenance as per schedule.',
        29, 'low', 'submitted', 0, now() - interval '30 minutes')
ON CONFLICT (id) DO NOTHING;

-- ── Workflow nodes for req_demo_001 ───────────────────────────────────
INSERT INTO workflow_nodes (id, request_id, key, name, agent_type, department, status, description, progress_percent, status_text, started_at)
VALUES ('wn_demo_001', 'req_demo_001', 'inspect', 'Inspect Compressor',
        'operations', 'Operations', 'completed', 'Initial inspection completed', 100, 'Done',
        now() - interval '3 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO workflow_nodes (id, request_id, key, name, agent_type, department, status, description, progress_percent, status_text, started_at)
VALUES ('wn_demo_002', 'req_demo_001', 'diagnose', 'Diagnose Vibration Issue',
        'maintenance', 'Maintenance', 'in_progress', 'Diagnosing root cause of vibration', 60, 'Analyzing data',
        now() - interval '2 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO workflow_nodes (id, request_id, key, name, agent_type, department, status, description, progress_percent, status_text)
VALUES ('wn_demo_003', 'req_demo_001', 'repair', 'Schedule Repair',
        'maintenance', 'Maintenance', 'pending', 'Awaiting diagnosis results', 0, 'Pending diagnosis')
ON CONFLICT (id) DO NOTHING;

-- ── Agent tasks for workflow nodes ────────────────────────────────────
INSERT INTO agent_tasks (id, node_id, title, status, ordinal, started_at, created_at)
VALUES ('at_demo_001', 'wn_demo_001', 'Visually inspect Compressor A for damage', 'completed', 0,
        now() - interval '3 hours', now() - interval '3 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO agent_tasks (id, node_id, title, status, ordinal, started_at, created_at)
VALUES ('at_demo_002', 'wn_demo_001', 'Record vibration readings at 3 points', 'completed', 1,
        now() - interval '2 hours', now() - interval '3 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO agent_tasks (id, node_id, title, status, ordinal, started_at)
VALUES ('at_demo_003', 'wn_demo_002', 'Analyze vibration spectrum data', 'in_progress', 0,
        now() - interval '1 hour')
ON CONFLICT (id) DO NOTHING;

INSERT INTO agent_tasks (id, node_id, title, status, ordinal)
VALUES ('at_demo_004', 'wn_demo_002', 'Compare with baseline readings', 'pending', 1)
ON CONFLICT (id) DO NOTHING;

-- ── Audit events ─────────────────────────────────────────────────────
INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
VALUES ('aev_demo_001', 'req_demo_001', 'wn_demo_001', 'Dana Founder', 'request.submitted',
        'Reported vibration issue on Compressor A', now() - interval '3 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
VALUES ('aev_demo_002', 'req_demo_001', 'wn_demo_001', 'Otto Ops', 'node.completed',
        'Initial inspection completed — found abnormal wear on bearing', now() - interval '2 hours')
ON CONFLICT (id) DO NOTHING;

INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
VALUES ('aev_demo_003', 'req_demo_001', 'wn_demo_002', 'Tara Technician', 'node.in_progress',
        'Started diagnostic analysis of vibration data', now() - interval '1 hour')
ON CONFLICT (id) DO NOTHING;

INSERT INTO audit_events (id, request_id, node_id, actor, action, reason, created_at)
VALUES ('aev_demo_004', 'req_demo_001', 'wn_demo_002', 'System', 'progress.updated',
        'Diagnosis 60% complete — awaiting spectrum analysis', now() - interval '30 minutes')
ON CONFLICT (id) DO NOTHING;
