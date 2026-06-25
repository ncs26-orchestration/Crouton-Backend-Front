# Pablo — Implementation Phase Specification

## User Stories Mapping

### Phase 1: Core Platform Structure ✅

| ID | User Story | Status |
|----|-----------|--------|
| US-001 | As an integrator, I want a workspace per client organisation, so I can manage workflows separately | ✅ |
| US-002 | As an integrator, I want to create a new client workspace, so I can start a new engagement | ✅ |
| US-003 | As an integrator, I want to list all my client organisations, so I can navigate between workspaces | ✅ |

**Completed Features:**
- Project CRUD (create, list, get, delete)
- Chat CRUD per project
- Message persistence per chat
- Basic UI: shell, project tree, chat view

---

### Phase 2: Organisation Onboarding ✅

| ID | User Story | Status |
|----|-----------|--------|
| US-004 | As an integrator, I want to start an onboarding interview for a new organisation, so I can understand their context | ✅ |
| US-005 | As an integrator, I want the AI to ask objective questions (not subjective), so I get factual answers | ✅ |
| US-006 | As an integrator, I want the AI to build an organisation overview from interview responses, so I have a factual baseline | ✅ |

**Completed Features:**
- Deterministic Onboarding Flow: 8 fixed questions (name, industry, size, regions, systems, processes, compliance, languages)
- Objective Questions Only: Multi-choice with "Other" and "None" options → factual answers
- Project Creation at End: Project only created when onboarding completes or skipped
- Chip/Tag Input: Type + Enter to create custom chips, click X to remove
- Organisation Settings Modal: Accessed from ProjectTree header when project scoped
- Go API: GET /onboarding, POST /onboard endpoints
- Web UI: OnboardingWizard component with ChipInput

---

### Phase 3: Conversation → Workflow Pipeline ✅

| ID | User Story | Status |
|----|-----------|--------|
| US-009 | As an integrator, I want to start a conversation about a process, so I can explore workflow ideas | ✅ |
| US-010 | As an integrator, I want each prompt to either modify the workflow OR ask clarifying questions, so ambiguity is eliminated | ✅ |
| US-011 | As an integrator, I want the AI to ask multiple questions until fully resolved, so no ambiguous nodes exist in final workflow | ✅ |
| US-012 | As an integrator, I want to preview the workflow as we chat, so I see changes in real-time | ✅ |
| US-013 | As an integrator, I want to approve the workflow when it's complete, so we move to formalization | ✅ |

**Completed Features:**
- Message → Extractor call (one-call: IR + questions)
- Workflow IR schema (actors, tasks, gateways, flows, events)
- Canvas rendering with React Flow
- Low-confidence questions inline (clarify loop)
- Refinement prompts (modify IR without re-extracting)
- Compile to BPMN 2.0 / Elsa 3
- Approve workflow → stage flips to approved
- Multi-source data: PDF/TXT attachments as extractor context

---

### Phase 4: Workflow Versioning & Forking ⏳

| ID | User Story | Status |
|----|-----------|--------|
| US-014 | As an integrator, I want to version a workflow, so I can track changes over time | ⏳ |
| US-015 | As an integrator, I want to fork an existing workflow, so I can create variants without losing original | ⏳ |
| US-016 | As an integrator, I want to see a diff between versions, so I understand what changed | ⏳ |
| US-017 | As an integrator, I want to restore a previous version, so I can rollback if needed | ⏳ |

---

### Phase 5: RAG & Confidence ⚠️

| ID | User Story | Status |
|----|-----------|--------|
| US-018 | As an integrator, I want sentences from onboarding to be stored with embeddings, so AI can cite sources | ⏳ |
| US-019 | As an integrator, I want every workflow node to show confidence score + evidence, so I know what needs verification | ⚠️ Partial |
| US-020 | As an integrator, I want to flag ambiguous nodes, so they can be resolved before deployment | ⏳ |

**Partial:**
- confidence + evidence fields exist in IR schema (apps/api/internal/ir/types.go)
- compiler propagates confidence through bindings
- UI does not yet surface confidence scores visually

---

### Phase 6: Elsa 3 Execution ✅

| ID | User Story | Status |
|----|-----------|--------|
| US-021 | As an integrator, I want to deploy a workflow to Elsa 3, so it can execute | ✅ |
| US-022 | As an integrator, I want a mock activity dialog, so I can simulate human tasks during testing | ⏳ |
| US-023 | As an integrator, I want to see execution history, so I can debug issues | ⏳ |

**Completed Features:**
- Deploy to Elsa 3 with real artifact push
- Credentials auth (basic, bearer, apikey)
- Toast with clickable Elsa Studio link
- Artifact wrapped in `{model, publish}` envelope

---

## Non-Functional Requirements

| ID | Requirement | Status |
|----|------------|--------|
| NFR-001 | Objective analysis only — AI never expresses opinions, only cites facts from onboarding/interview | ✅ |
| NFR-002 | Zero ambiguity — No workflow can be exported until all nodes resolved | ⏳ |
| NFR-003 | Platform isolation — Integrator never needs client credentials; all data extracted/forked | ✅ |
| NFR-004 | Multi-modal input — Support documents, images, voice from day 1 | ⚠️ Partial |

---

## Technical Notes

### Deterministic Questions Schema (Phase 2)

```go
Questions: []Question{
  { ID: "org_name",    Type: "text",   Label: "Organisation Name", Required: true },
  { ID: "industry",    Type: "select", Label: "Industry", Options: [...] },
  { ID: "company_size",Type: "select", Label: "Company Size", Options: [...] },
  { ID: "regions",    Type: "multi",   Label: "Operating Regions", Options: [...] },
  { ID: "systems",    Type: "multi",   Label: "Business Systems", Options: [...] },
  { ID: "processes",  Type: "multi",   Label: "Main Processes", Options: [...] },
  { ID: "compliance", Type: "multi",   Label: "Compliance", Options: [...] },
  { ID: "languages",  Type: "multi",   Label: "Languages", Options: [...] },
}
```

### Organisation Overview Storage

- Stored in `projects.overview_json` as JSONB
- Fields: name, industry, size, regions[], systems[], processes[], compliance[], languages[]
- Fetched via GET /projects/{id} → used as AI context in extractor prompts

### Workflow IR Schema (Phase 3)

Every element carries:
- `confidence` (float, 0.0–1.0)
- `evidence` (string — quoted source text)

Elements: actors, tasks, gateways, events, flows, forms.

### Database Migration (Phase 2)

```sql
ALTER TABLE chats ADD COLUMN kind TEXT NOT NULL DEFAULT 'workflow';
ALTER TABLE projects ADD COLUMN overview_json JSONB;
```

### Elsa 3 Deploy Contract (Phase 6)

```json
{
  "model": { /* BPMN 2.0 XML */ },
  "publish": true
}
```