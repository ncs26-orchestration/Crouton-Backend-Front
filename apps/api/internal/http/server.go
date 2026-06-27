package http

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/agentclient"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/camunda7"
	"github.com/ncs26-orchestration/solution/apps/api/internal/engine/elsa3"
	"github.com/ncs26-orchestration/solution/apps/api/internal/handler"
	authmw "github.com/ncs26-orchestration/solution/apps/api/internal/middleware"
	"github.com/ncs26-orchestration/solution/apps/api/internal/orchestrator"
	"github.com/ncs26-orchestration/solution/apps/api/internal/repo"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Logger          *slog.Logger
	PgPool          *pgxpool.Pool
	Redis           *redis.Client
	AgentURL        string
	JWTSecret       string
	OrchStepDelayMS int
	// RootCtx ties orchestration workers to the server lifetime so they stop
	// on shutdown. Defaults to context.Background() when nil.
	RootCtx context.Context
}

func NewServer(d Deps) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	}))
	e.Use(middleware.RequestLogger())

	h := handler.New(d.Logger, d.PgPool, d.Redis)
	e.GET("/healthz", h.Health)
	e.GET("/readyz", h.Ready)

	// Auth — no middleware required.
	ah := handler.NewAuthHandler(d.Logger, d.PgPool, d.JWTSecret)
	e.POST("/auth/register", ah.Register)
	e.POST("/auth/login", ah.Login)
	e.GET("/users/lookup", ah.LookupUser)

	// Orgs, teams and members — all routes require a valid JWT.
	authMiddleware := authmw.NewAuthMiddleware(d.JWTSecret)
	oh := handler.NewOrgsHandler(d.Logger, d.PgPool)

	// POST /orgs is authenticated but open to any registered user.
	e.POST("/orgs", oh.CreateOrg, authMiddleware)

	orgGroup := e.Group("/orgs", authMiddleware)
	orgGroup.GET("", oh.ListOrgs)
	orgGroup.GET("/:orgId", oh.GetOrg)
	orgGroup.DELETE("/:orgId", oh.DeleteOrg)

	orgGroup.POST("/:orgId/teams", oh.CreateTeam)
	orgGroup.GET("/:orgId/teams", oh.ListTeams)
	orgGroup.GET("/:orgId/teams/:teamId", oh.GetTeam)
	orgGroup.PATCH("/:orgId/teams/:teamId", oh.UpdateTeam)
	orgGroup.DELETE("/:orgId/teams/:teamId", oh.DeleteTeam)

	orgGroup.POST("/:orgId/members", oh.AddOrgMember)
	orgGroup.POST("/:orgId/members/invite", oh.InviteMember)
	orgGroup.GET("/:orgId/members", oh.ListOrgMembers)
	orgGroup.PATCH("/:orgId/members/:userId", oh.UpdateOrgMemberRole)
	orgGroup.DELETE("/:orgId/members/:userId", oh.RemoveOrgMember)

	orgGroup.POST("/:orgId/teams/:teamId/members", oh.AddTeamMember)
	orgGroup.DELETE("/:orgId/teams/:teamId/members/:userId", oh.RemoveTeamMember)

	// Agents and policies read endpoints (F10).
	orgGroup.GET("/:orgId/agents", oh.ListAgents)
	orgGroup.GET("/:orgId/policies", oh.ListPolicies)
	orgGroup.POST("/:orgId/policies", oh.CreatePolicy)
	orgGroup.PATCH("/:orgId/policies/:policyId", oh.UpdatePolicy)
	orgGroup.DELETE("/:orgId/policies/:policyId", oh.DeletePolicy)

	// Me — current user's profile and work items.
	mh := handler.NewMeHandler(d.Logger, d.PgPool)
	e.GET("/me", mh.GetMeProfile, authMiddleware)
	e.GET("/me/work", mh.GetMyWork, authMiddleware)

	// Requests — submission, listing, detail with the workflow graph, and
	// node detail. The intake planner turns a request into a department
	// workflow; the orchestration engine then runs each node through its
	// department agent on a background worker (F3).
	agentClient := agentclient.New(d.AgentURL)
	auditRepo := repo.NewAuditRepo(d.PgPool)

	// SSE event bus — shared between the engine (publishes) and the events
	// handler (subscribes + streams to the browser).
	eventBus := orchestrator.NewBus()

	docRepo := repo.NewDocumentRepo(d.PgPool)
	store := orchestrator.NewDBStore(
		repo.NewRequestRepo(d.PgPool),
		repo.NewWorkflowRepo(d.PgPool),
		auditRepo,
		repo.NewDependencyRepo(d.PgPool),
		docRepo,
		repo.NewPolicyRepo(d.PgPool),
		repo.NewAssignmentRepo(d.PgPool),
	)
	rootCtx := d.RootCtx
	if rootCtx == nil {
		rootCtx = context.Background()
	}
	orchEngine := orchestrator.NewEngine(rootCtx, d.Logger, store, agentClient,
		time.Duration(d.OrchStepDelayMS)*time.Millisecond, eventBus)
	// Resume any request a prior run left mid-orchestration (restart recovery).
	go orchEngine.ResumeInProgress()
	reqh := handler.NewRequestsHandler(d.Logger, d.PgPool, agentClient, orchEngine)
	orgGroup.POST("/:orgId/requests", reqh.CreateRequest)
	orgGroup.GET("/:orgId/requests", reqh.ListRequests)
	orgGroup.GET("/:orgId/my-verifications", reqh.MyVerifications)
	// Reusable internal workflows (definitions + on-demand runs).
	wfh := handler.NewWorkflowsHandler(d.Logger, d.PgPool)
	orgGroup.GET("/:orgId/workflows", wfh.ListWorkflows)
	orgGroup.POST("/:orgId/workflows", wfh.CreateWorkflow)
	orgGroup.PATCH("/:orgId/workflows/:id", wfh.UpdateWorkflow)
	orgGroup.DELETE("/:orgId/workflows/:id", wfh.DeleteWorkflow)
	orgGroup.GET("/:orgId/workflows/:id/runs", wfh.ListWorkflowRuns)
	orgGroup.POST("/:orgId/workflows/:id/run", wfh.RunWorkflow)
	e.GET("/requests/:id", reqh.GetRequest, authMiddleware)
	e.GET("/requests/:id/nodes/:nodeId", reqh.GetNode, authMiddleware)
	// Executive approval gate (F7): an approver decides a request parked at
	// awaiting_approval; approve resumes the worker, reject stops it.
	e.POST("/requests/:id/approve", reqh.ApproveRequest, authMiddleware)
	// Human-in-the-loop: customize a draft (assign verifiers), launch it, and
	// verify a node parked at awaiting_review.
	e.POST("/requests/:id/assignments", reqh.AssignNode, authMiddleware)
	e.DELETE("/requests/:id/assignments/:assignmentId", reqh.UnassignNode, authMiddleware)
	e.POST("/requests/:id/launch", reqh.LaunchRequest, authMiddleware)
	e.PUT("/requests/:id/graph", reqh.UpdateRequestGraph, authMiddleware)
	e.POST("/requests/:id/nodes/:nodeId/verify", reqh.VerifyNode, authMiddleware)
	// Per-node verifier↔agent conversation (ask questions / request changes).
	e.GET("/requests/:id/nodes/:nodeId/messages", reqh.ListNodeMessages, authMiddleware)
	e.POST("/requests/:id/nodes/:nodeId/messages", reqh.PostNodeMessage, authMiddleware)
	// Audit reads (F6).
	e.GET("/requests/:id/audit", reqh.ListRequestAudit, authMiddleware)
	orgGroup.GET("/:orgId/audit", reqh.ListOrgAudit)

	// Documents — auto-generated completion summaries and manual uploads.
	doch := handler.NewDocumentsHandler(d.Logger, d.PgPool)
	e.GET("/requests/:id/documents", doch.ListDocuments, authMiddleware)
	e.POST("/requests/:id/documents", doch.UploadDocument, authMiddleware)
	e.GET("/documents/:id/download", doch.DownloadDocument, authMiddleware)

	// SSE events endpoint — authenticates via ?token= so EventSource can
	// connect (it cannot set custom headers). The auth middleware is not
	// used here; the handler reads the token from the query parameter.
	evh := handler.NewEventsHandler(d.Logger, d.PgPool, d.JWTSecret, eventBus)
	e.GET("/requests/:id/events", evh.Stream)

	// Final report (F8) — compiled on-the-fly from the request, nodes,
	// tasks, and audit trail. No separate report table.
	rpt := handler.NewReportHandler(d.Logger, d.PgPool)
	e.GET("/requests/:id/report", rpt.GetReport, authMiddleware)

	// Mobile-app work endpoints: inbox, task detail, task actions.
	wh := handler.NewWorkHandler(d.Logger, d.PgPool)
	e.GET("/me/work", wh.ListMyWork, authMiddleware)
	e.GET("/tasks/:id/detail", wh.GetTaskDetail, authMiddleware)
	e.POST("/tasks/:id/progress", wh.PostTaskProgress, authMiddleware)
	e.POST("/tasks/:id/complete", wh.PostTaskComplete, authMiddleware)
	e.POST("/tasks/:id/decision", wh.PostTaskDecision, authMiddleware)

	// Machine registry (M-F1) — equipment owned by the org. Technician
	// RBAC is enforced inside the handler (technician sees only assigned).
	machH := handler.NewMachinesHandler(d.Logger, d.PgPool, d.JWTSecret)
	orgGroup.POST("/:orgId/machines", machH.CreateMachine)
	orgGroup.GET("/:orgId/machines", machH.ListMachines)
	e.GET("/machines/:id", machH.GetMachine, authMiddleware)
	e.PUT("/machines/:id", machH.UpdateMachine, authMiddleware)
	// Telemetry webhook — no auth middleware (handler handles both JWT
	// and machine API key auth internally).
	e.POST("/machines/:id/telemetry", machH.PostTelemetry)
	e.GET("/machines/:id/telemetry", machH.ListTelemetry, authMiddleware)

	// Incidents (M-F6) — technician-reported problems on machines. Creating an
	// incident auto-flips machine status to "down"; resolving flips it back.
	incH := handler.NewIncidentsHandler(d.Logger, d.PgPool, d.JWTSecret)
	orgGroup.GET("/:orgId/incidents", incH.ListIncidents, authMiddleware)
	e.POST("/incidents", incH.CreateIncident, authMiddleware)
	e.GET("/incidents/:id/messages", incH.ListMessages, authMiddleware)
	e.POST("/incidents/:id/messages", incH.AppendMessage, authMiddleware)
	e.POST("/incidents/:id/resolve", incH.ResolveIncident, authMiddleware)
	e.GET("/incidents/:id/events", incH.StreamEvents)

	// Diagnosis — AI-powered machine incident diagnostics. Technicians
	// request a diagnosis and follow step-by-step repair checkpoints.
	diagH := handler.NewDiagnosisHandler(d.Logger, d.PgPool, agentClient)
	e.POST("/machines/:id/documents", diagH.UploadMachineDocument, authMiddleware)
	e.GET("/machines/:id/documents", diagH.ListMachineDocuments, authMiddleware)
	e.POST("/incidents/:id/diagnose", diagH.RequestDiagnosis, authMiddleware)
	e.GET("/incidents/:id/diagnosis", diagH.GetDiagnosis, authMiddleware)
	e.POST("/diagnosis/steps/:stepId/complete", diagH.CompleteStep, authMiddleware)

	// Engine-adapter registry. Each adapter implements the
	// engine.Adapter interface; the registry is the single lookup
	// the HTTP layer uses to route /compile/:target, and the source
	// of truth for GET /engines/adapters that populates the web
	// target selector.
	adapters := engine.NewRegistry()
	adapters.Register(camunda7.NewAdapter())
	adapters.Register(elsa3.NewAdapter())

	ch, err := handler.NewCompileHandler(d.Logger, adapters)
	if err != nil {
		// A failure here means the embedded IR schemas are malformed at
		// build time — fatal, not recoverable at runtime.
		d.Logger.Error("init compile handler", slog.String("err", err.Error()))
		panic(err)
	}
	// Legacy route — kept so curl smoke tests and existing clients
	// keep working. It's an alias for POST /compile/camunda7.
	e.POST("/compile/bpmn", ch.CompileBPMN)
	e.POST("/compile/:target", ch.CompileForTarget)
	e.GET("/engines/adapters", ch.ListAdapters)
	e.POST("/analyze/decision-tables", ch.AnalyzeDecisionTables)

	eh := handler.NewEnginesHandler(d.Logger, d.PgPool)
	e.POST("/engines", eh.CreateEngine)
	e.POST("/engines/:id/sync", eh.SyncEngine)
	e.GET("/is", eh.ListIS)
	e.POST("/systems", eh.DeclareSystem)

	xh, err := handler.NewExtractHandler(d.Logger, d.PgPool, d.AgentURL)
	if err != nil {
		d.Logger.Error("init extract handler", slog.String("err", err.Error()))
		panic(err)
	}
	e.POST("/extract", xh.Extract)

	dh, err := handler.NewDeployHandler(d.Logger, d.PgPool)
	if err != nil {
		d.Logger.Error("init deploy handler", slog.String("err", err.Error()))
		panic(err)
	}
	e.POST("/deploy/camunda", dh.DeployBPMN)
	e.DELETE("/engines/:id", dh.DeleteEngine)

	rh := handler.NewDeploymentsHandler(d.Logger, d.PgPool)
	e.GET("/engines/:id/runs", rh.ListRuns)
	e.POST("/engines/:id/processes/:key/start", rh.StartInstance)

	coh, err := handler.NewCopilotHandler(d.Logger, d.PgPool, d.AgentURL)
	if err != nil {
		d.Logger.Error("init copilot handler", slog.String("err", err.Error()))
		panic(err)
	}
	e.POST("/copilot/ask", coh.Ask)
	e.POST("/copilot/clarify", coh.Clarify)
	e.POST("/copilot/apply", coh.Apply)

	// Projects + Chats + Messages — new top-level navigation for the
	// operator-tool repositioning. Rounds A + B ship CRUD and
	// navigation; Round C wires extraction to /chats/:id/messages.
	ph, err := handler.NewProjectsHandler(d.Logger, d.PgPool, d.AgentURL)
	if err != nil {
		d.Logger.Error("init projects handler", slog.String("err", err.Error()))
		panic(err)
	}
	orgGroup.POST("/:orgId/projects", ph.CreateProject)
	orgGroup.GET("/:orgId/projects", ph.ListProjects)
	e.GET("/projects/:id", ph.GetProject)
	e.PATCH("/projects/:id", ph.UpdateProject)
	e.DELETE("/projects/:id", ph.ArchiveProject)
	e.POST("/projects/:id/chats", ph.CreateChat)
	e.GET("/projects/:id/chats", ph.ListChats)
	e.GET("/chats/:id", ph.GetChat)
	e.PATCH("/chats/:id", ph.RenameChat)
	e.DELETE("/chats/:id", ph.DeleteChat)
	e.POST("/chats/:id/messages", ph.AppendMessage)
	e.GET("/chats/:id/messages", ph.ListMessages)
	e.POST("/chats/:id/approve", ph.ApproveWorkflow)

	// Workflow Versioning
	e.GET("/chats/:id/workflow/versions", ph.ListWorkflowVersions)
	e.GET("/workflow-versions/:id", ph.GetWorkflowVersion)
	e.POST("/chats/:id/workflow/fork", ph.ForkWorkflow)
	e.POST("/workflow-versions/:id/restore", ph.RestoreWorkflowVersion)
	e.POST("/workflow-versions/:id/diff", ph.DiffWorkflowVersions)

	// Onboarding wizard — multi-step questionnaire that builds
	// projects.overview_json. GET returns questions, POST handles answers.
	e.GET("/onboarding", ph.GetOnboardingQuestions)
	e.POST("/projects/:id/onboarding", ph.OnboardProject)
	e.PATCH("/projects/:id/overview", ph.UpdateProjectOverview)

	// Attachments — multipart upload proxied to the agent for text
	// extraction, then persisted as text_content so the extractor's
	// chat_context can consume it on the next message.
	ath := handler.NewAttachmentsHandler(d.Logger, d.PgPool, d.AgentURL)
	e.POST("/chats/:id/attachments", ath.Upload)
	e.GET("/chats/:id/attachments", ath.List)

	// Deploy targets — project-scoped replacement for the old
	// tenant-wide engine_connections UX. The handler also owns the
	// chat-scoped /deploy endpoint, which compiles + pushes the
	// chat's latest IR through the adapter for the selected target.
	dth := handler.NewDeployTargetsHandler(d.Logger, d.PgPool, adapters)
	e.POST("/projects/:id/deploy-targets", dth.Create)
	e.GET("/projects/:id/deploy-targets", dth.List)
	e.DELETE("/deploy-targets/:id", dth.Delete)
	e.POST("/chats/:id/deploy", dth.DeployChat)

	return e
}
