# AUP jury deck — outline

3 minutes live + 2 minutes Q&A. Mirrors VISION.md §9 demo script.
Pass this file to `/slidev:generate` once the plugin is installed.

## Target

- Jury: Innovia ESI / Algiers'UP / ETIC
- Duration: 3:00 live demo, 10 slides max
- Language: FR primary, EN subtitles in speaker notes
- Output: slides.md → PDF handout + live presenter mode

## Slide-by-slide

### 1. Title — 10s
- One sentence: **Unstructured → Bound → Portable**
- Sub: "An engine-agnostic workflow compiler"
- Background: dark brand (`#1c1e54`), ruby-to-magenta gradient accent

### 2. The problem — 20s
- Jury's own question (quoted): *"Comment passer d'un besoin métier non structuré à un workflow clair, cohérent et exploitable ?"*
- Two constraints: engine-agnostic + sobriety
- Evidence: 3 inputs (paragraph, sketch, transcript) shown as thumbnails

### 3. The pitch in one diagram — 20s
- The architecture block from VISION.md §3: input → understanding pipeline → IR → 4 compilers
- Animated: beam splits from IR into BPMN / Elsa / n8n / DMN
- Callout: "The engine is a plugin, not a dependency"

### 4. Act I — Onboard the IS — 25s *(live demo starts)*
- Screen: Onboarding view
- Click: OpenBee + O365 + LDAP + Odoo
- Result: IS Registry with 4 systems, capabilities indexed

### 5. Act II — Describe the process — 40s *(live)*
- Paste French transcript + upload whiteboard photo
- LangGraph pipeline runs (show the 8 nodes pulsing)
- Draft IR appears with actors/tasks/decisions

### 6. Act III — Bind & compile — 35s *(live)*
- Bindings auto-resolved (OpenBee for archive, O365 for email)
- 2 gap questions answered
- Click **Compile → Camunda 8** → BPMN opens in Camunda Modeler tab

### 7. Switch target — 20s *(live, the mic-drop)*
- Click **Switch target → Elsa**
- Same IR, now Elsa JSON, deployed
- Show `htop`: the whole run was 7B model local, no network egress

### 8. Why small models work — 25s
- IS Registry injected into constrained-decoding grammar
- Capabilities = allowed values → no hallucinated integrations
- One line: "A 7B grounded beats a 70B ungrounded"

### 9. Standing on standards — 15s
- Logo wall: BPMN 2.0, DMN, OpenAPI, OAuth2, LDAP, MCP, Apache Camel
- "We ride on standards, not reinvention"

### 10. Closing — 10s
- Tagline (whichever English name you picked — Score / Prism / Kiln)
- One sentence from VISION.md §13
- Contact + repo link

## Appendix slides (ready if jury asks)

- A1. IR lifecycle state machine (SYSTEM.md §5)
- A2. Engine capability profiles + lowering passes (SYSTEM.md §8.1)
- A3. Graceful degradation — human stub fallback
- A4. Round-trip: IR is source of truth
- A5. Local relevance — OpenBee + FR/AR first-class
- A6. Hackathon 72h plan (VISION.md §11)

## Screenshots to capture from the running app

Save under `apps/deck/public/screens/`:

- `01-onboarding.png` — empty IS Registry state
- `02-is-registry-filled.png` — after picking 4 systems
- `03-composer-paste.png` — transcript pasted, photo uploaded
- `04-ir-canvas-draft.png` — IR graph rendered, bindings populated
- `05-gap-inbox.png` — the 2 gap questions
- `06-compile-diff.png` — BPMN side-by-side with Camunda Modeler
- `07-switch-target.png` — same IR → Elsa view
- `08-htop.png` — local inference proof

## Theming — Stripe skin from DESIGN.md

See `theme/stripe/` — CSS variables already map to:
- Primary `#533afd`, Deep navy `#061b31`, Body `#64748d`
- sohne-var weight 300 for headlines, tracking tightens at display sizes
- Blue-tinted shadows `rgba(50,50,93,0.25)`
- 4–8px radii, never pills

## Build commands (once plugin installed)

```bash
cd apps/deck
/slidev:init            # scaffolds package.json + slides.md
/slidev:frame           # set duration/audience
/slidev:generate        # pulls this outline → slides.md
/slidev:preview         # opens http://localhost:3030
/slidev:export pdf      # produces apps/deck/slides-export.pdf
```
