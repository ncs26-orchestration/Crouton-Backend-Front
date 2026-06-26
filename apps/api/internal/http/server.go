package http

import (
	"log/slog"

	"github.com/Noussour/aup/apps/api/internal/engine"
	"github.com/Noussour/aup/apps/api/internal/engine/camunda7"
	"github.com/Noussour/aup/apps/api/internal/engine/elsa3"
	"github.com/Noussour/aup/apps/api/internal/handler"
	authmw "github.com/Noussour/aup/apps/api/internal/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	Logger    *slog.Logger
	PgPool    *pgxpool.Pool
	Redis     *redis.Client
	AgentURL  string
	JWTSecret string
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
	orgGroup.GET("/:orgId/members", oh.ListOrgMembers)
	orgGroup.PATCH("/:orgId/members/:userId", oh.UpdateOrgMemberRole)
	orgGroup.DELETE("/:orgId/members/:userId", oh.RemoveOrgMember)

	orgGroup.POST("/:orgId/teams/:teamId/members", oh.AddTeamMember)
	orgGroup.DELETE("/:orgId/teams/:teamId/members/:userId", oh.RemoveTeamMember)

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
