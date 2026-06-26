package ir

// ExecutableIR is a workflow that has passed through Lower: actors
// resolved to concrete user/group ids, default gateway branches
// synthesized, conditions normalized to an engine-ready expression
// language. It is the form the compilers (BPMN, Elsa, n8n, DMN)
// consume directly.
//
// Compilers may rely on the following invariants of an ExecutableIR
// (which Lower establishes or flags):
//
//   - Every exclusive gateway has at least one outgoing flow without
//     a condition — the "default" path, synthesized if the source
//     didn't provide one.
//   - Every user task's binding, if the task's actor has an is_ref,
//     has either assignee_user_id or candidate_group_id populated.
//   - Every flow.condition has a non-empty language (default "juel").
//   - Confidence values are propagated so a task reads as its own
//     minimum of (task.confidence, binding.confidence).
//
// ExecutableIR is a structural alias for Workflow; the distinction
// is semantic. See process.go for the pair.
type ExecutableIR = Workflow
