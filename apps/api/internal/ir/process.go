package ir

// ProcessIR is the business-layer representation a workflow takes
// coming out of the extractor: abstract actors that may or may not
// have is_ref pointers, free-form conditions, bindings that can be
// partial or missing entirely, and potentially ambiguous elements
// (low-confidence, missing default branches, unresolved capabilities).
//
// Structurally, a ProcessIR is a Workflow — the JSON shape is
// identical. The distinction is semantic and enforced by the
// compiler pipeline: ProcessIR is the INPUT to Lower; ExecutableIR
// is the OUTPUT. Keeping them as type aliases of the same concrete
// type avoids a duplicated struct while letting function signatures
// document the stage they operate in.
//
// The extractor emits ProcessIR. The compilers (BPMN, Elsa, etc.)
// consume ExecutableIR. Lower sits between them.
type ProcessIR = Workflow
