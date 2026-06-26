# Vendored skills

Curated engineering and design skills for the AI Organization OS pivot, vendored into the repo so
any coding agent (not just one vendor's CLI) can use them. They are reference playbooks: read the
relevant `SKILL.md` before doing work in that discipline.

Grouped by team:

- `frontend/` — React 19 + Vite + React Flow + Tailwind UI work and design taste
- `backend/` — Go 1.25 + Echo + pgx API and the orchestration engine
- `agent/` — the Pydantic AI agent layer (typed agents, tools, multi-agent design)

Each subfolder is a self-contained skill (`SKILL.md` plus any `reference/` material). Heavy assets
(`scripts/`, `evals/`, images) were excluded when vendoring to keep the repo lean.

## frontend

- **i-impeccable** — design, redesign, audit, and polish interfaces (UX, hierarchy, motion, a11y)
- **emil-design-eng** — Emil Kowalski's philosophy on UI polish, component and animation detail
- **design-taste-frontend** — anti-slop frontend: infer the right design direction, avoid templated looks
- **high-end-visual-design** — agency-grade fonts, spacing, shadows, card structure that feel expensive
- **i-frontend-design** — frontend designer-engineer mindset, not a layout generator
- **i-web-design-guidelines** — review files against web interface guidelines
- **i-vercel-react-best-practices** — React/Next performance patterns from Vercel Engineering
- **i-react-flow-architect** — production React Flow apps: navigation, performance, state management
- **i-react-flow-node-ts** — React Flow node components with proper TS types and store integration
- **i-react-patterns** — modern React: hooks, composition, performance, TypeScript
- **i-react-state-management** — Redux Toolkit / Zustand / Jotai / React Query; server vs global state
- **i-tailwind-design-system** — design tokens, component variants, responsive + accessible Tailwind
- **i-frontend-api-integration-patterns** — race conditions, cancellation, retries, streaming/SSE
- **react-doctor** — scan/triage/clean React diagnostics (lint, a11y, bundle, architecture)

## backend

- **i-golang-pro** — modern Go: advanced concurrency, performance, production microservices
- **i-golang-project-layout** — Go project/workspace layout and organization
- **i-golang-code-style** — Go style, formatting, conventions, comments
- **i-golang-naming** — Go naming: packages, constructors, structs, interfaces, errors, receivers
- **i-golang-structs-interfaces** — composition, embedding, interface segregation, DI via interfaces
- **i-golang-design-patterns** — functional options, lifecycle, graceful shutdown, resilience
- **i-golang-concurrency** — goroutines, channels, select, errgroup, worker pools (the orchestrator)
- **i-golang-context** — context creation, propagation, cancellation, deadlines, cross-service tracing
- **i-golang-error-handling** — wrapping with %w, errors.Is/As, sentinel errors, the single-handling rule
- **i-golang-database** — pgx/database access: parameterized queries, scanning, NULLs, transactions
- **i-golang-testing** — table-driven tests, suites, mocks, unit + integration
- **i-golang-stretchr-testify** — assert/require/mock/suite in depth
- **i-golang-lint** — golangci-lint config and practices
- **i-golang-security** — injection, crypto, filesystem/network safety
- **i-golang-observability** — slog structured logging, Prometheus metrics, OpenTelemetry tracing
- **i-api-design-principles** — REST/GraphQL API design for intuitive, scalable, maintainable APIs

## agent

- **i-pydantic-ai** — production Pydantic AI agents: type-safe tool use, structured output, DI, multi-model
- **i-pydantic-models-py** — Pydantic multi-model pattern for clean API contracts (agent I/O models)
- **i-agent-tool-builder** — designing the tools agents use to interact with the world
- **i-ai-agents-architect** — designing and building autonomous AI agents, tool design
- **i-multi-agent-architect** — production multi-agent systems with LangGraph/LangChain/DeepAgents
- **i-multi-agent-patterns** — supervisor, swarm, and coordination patterns for multiple agents
- **i-autonomous-agent-patterns** — design patterns for autonomous agents
- **i-agent-evaluation** — testing and benchmarking LLM agents, behavioral testing
