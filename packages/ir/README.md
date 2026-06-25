# @repo/ir — AUP Workflow IR v0.1

Canonical, engine-agnostic JSON representation of a business workflow,
plus the schemas AUP binds against.

## Contents

| File | Purpose |
|---|---|
| `schema.json` | Workflow IR v0.1 JSON Schema (Draft 2020-12). Every IR produced by extraction or edited in the UI validates against this. |
| `is-registry.schema.json` | IS Registry v0.1 JSON Schema. Describes what AUP caches per tenant: projected users/groups/forms from the engine, and declared systems with capability catalogs. |
| `engine-profiles/camunda7.json` | Capability profile for Camunda 7. Read by the BPMN compiler to pick the right XML shape (namespaces, JUEL vs FEEL, external-task topics, embedded formData). |
| `examples/` | Three example IRs used by golden tests and the demo. |

## Identity invariant

The IR is the only place in AUP that owns workflow structure. Identities
— users, groups, deployed forms — belong to the engine and the client's
IdP. AUP projects them read-only into the IS Registry and the IR only
**references** them. If a binding's `assignee_user_id`, `candidate_group_id`,
`system_ref`, `capability`, or `form_key` does not resolve against a
supplied IS Registry, the workflow is invalid.

## IR shape (v0.1)

- `version`: `"0.1"`
- `metadata`: `{ name, description?, tenant_id?, source_ids[]? }`
- `actors[]`: `{ id, name, kind: "role"|"person"|"group", is_ref? }`
- `tasks[]`: `{ id, name, type: "user"|"service"|"script", actor_ref?, form_ref?, binding? }`
- `gateways[]`: `{ id, type: "exclusive"|"parallel" }`
- `events[]`: `{ id, type: "start"|"end" }`
- `flows[]`: `{ id, from, to, condition? }`
- `forms[]`: `{ id, fields[] }`

Reserved for later versions: `rules[]` (decision tables), richer event
types, boundary events, subprocesses, multi-instance.

## Binding shape

User tasks fill in:
- `assignee_user_id` — a user id from the projection.
- `candidate_group_id` — a group id from the projection.
- `form_key` — a deployed form key (projected) OR leave it off and use `form_ref` for an inline form.

Service tasks fill in:
- `system_ref` — a declared system id.
- `capability` — a capability declared by that system.
- `params` — expression-template values, resolved at engine runtime.

## Camunda 7 demo identities

`camunda/camunda-bpm-platform:run-latest` ships with four demo users
(`demo`, `john`, `mary`, `peter`) and four groups (`sales`, `accounting`,
`management`, `camunda-admin`). Memberships per Camunda's
`DemoDataGenerator.java`:

- `demo` → all groups
- `john` → `sales`
- `mary` → `accounting`
- `peter` → `management`

All example IRs reference those group ids so a compile + deploy produces
tasks routed to real inboxes.

## Validating an IR outside Go

```bash
# with `check-jsonschema`
pip install check-jsonschema
check-jsonschema \
  --schemafile packages/ir/schema.json \
  packages/ir/examples/expense-approval.json
```
