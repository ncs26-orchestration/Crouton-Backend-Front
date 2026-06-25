package handler

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"github.com/Noussour/aup/apps/api/internal/ir"
	"github.com/Noussour/aup/apps/api/internal/repo"
)

// ProjectsHandler owns all /projects + /chats + /messages +
// /attachments + /workflow-versions routes. They share repositories
// and are small enough to keep in one handler; splitting would add
// ceremony without clarity.
type ProjectsHandler struct {
	logger    *slog.Logger
	pg        *pgxpool.Pool
	projects  *repo.ProjectRepo
	chats     *repo.ChatRepo
	messages  *repo.ChatMessageRepo
	attach    *repo.AttachmentRepo
	versions  *repo.WorkflowVersionRepo
	validator *ir.Validator
	agentURL  string
	http      *http.Client
}

func NewProjectsHandler(logger *slog.Logger, pg *pgxpool.Pool, agentURL string) (*ProjectsHandler, error) {
	v, err := ir.NewValidator()
	if err != nil {
		return nil, err
	}
	return &ProjectsHandler{
		logger:    logger,
		pg:        pg,
		projects:  repo.NewProjectRepo(pg),
		chats:     repo.NewChatRepo(pg),
		messages:  repo.NewChatMessageRepo(pg),
		attach:    repo.NewAttachmentRepo(pg),
		versions:  repo.NewWorkflowVersionRepo(pg),
		validator: v,
		agentURL:  strings.TrimRight(agentURL, "/"),
		http:      &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// --- /projects ---

type projectRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type projectResponse struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	OverviewJSON json.RawMessage `json:"overview_json,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

func (h *ProjectsHandler) CreateProject(c echo.Context) error {
	var req projectRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name_required"})
	}
	p := repo.Project{
		ID:          makeID("p", name),
		Name:        name,
		Description: strings.TrimSpace(req.Description),
	}
	if err := h.projects.Create(c.Request().Context(), p); err != nil {
		h.logger.Error("create project", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.JSON(http.StatusCreated, projectResponse{
		ID: p.ID, Name: p.Name, Description: p.Description, CreatedAt: time.Now(),
	})
}

func (h *ProjectsHandler) ListProjects(c echo.Context) error {
	ps, err := h.projects.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	out := make([]projectResponse, 0, len(ps))
	for _, p := range ps {
		overview, _ := h.projects.GetOverview(c.Request().Context(), p.ID)
		out = append(out, projectResponse{ID: p.ID, Name: p.Name, Description: p.Description, OverviewJSON: overview, CreatedAt: p.CreatedAt})
	}
	return c.JSON(http.StatusOK, map[string]any{"projects": out})
}

func (h *ProjectsHandler) GetProject(c echo.Context) error {
	p, err := h.projects.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	chats, _ := h.chats.ListByProject(c.Request().Context(), p.ID)
	overview, _ := h.projects.GetOverview(c.Request().Context(), p.ID)
	return c.JSON(http.StatusOK, map[string]any{
		"project": projectResponse{ID: p.ID, Name: p.Name, Description: p.Description, OverviewJSON: overview, CreatedAt: p.CreatedAt},
		"chats":   mapChatsToResponse(chats),
	})
}

func (h *ProjectsHandler) UpdateProject(c echo.Context) error {
	var req projectRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	if err := h.projects.Update(c.Request().Context(), c.Param("id"), strings.TrimSpace(req.Name), strings.TrimSpace(req.Description)); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ProjectsHandler) UpdateProjectOverview(c echo.Context) error {
	ctx := c.Request().Context()
	projectID := c.Param("id")

	_, err := h.projects.Get(ctx, projectID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	var req struct {
		Overview json.RawMessage `json:"overview"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}

	if err := h.projects.SetOverview(ctx, projectID, req.Overview); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

func (h *ProjectsHandler) ArchiveProject(c echo.Context) error {
	if err := h.projects.Archive(c.Request().Context(), c.Param("id")); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.NoContent(http.StatusNoContent)
}

// --- /chats ---

type chatRequest struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type chatResponse struct {
	ID                      string    `json:"id"`
	ProjectID               string    `json:"project_id"`
	Kind                    string    `json:"kind"`
	Title                   string    `json:"title"`
	Summary                 string    `json:"summary"`
	LatestWorkflowVersionID *string   `json:"latest_workflow_version_id,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

func mapChatsToResponse(cs []repo.Chat) []chatResponse {
	out := make([]chatResponse, 0, len(cs))
	for _, c := range cs {
		out = append(out, chatResponse{
			ID: c.ID, ProjectID: c.ProjectID, Kind: c.Kind, Title: c.Title, Summary: c.Summary,
			LatestWorkflowVersionID: c.LatestWorkflowVersionID,
			CreatedAt:               c.CreatedAt, UpdatedAt: c.UpdatedAt,
		})
	}
	return out
}

func (h *ProjectsHandler) CreateChat(c echo.Context) error {
	projectID := c.Param("id")
	if _, err := h.projects.Get(c.Request().Context(), projectID); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "project_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	var req chatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "New chat"
	}
	ch := repo.Chat{
		ID:        makeID("c", title),
		ProjectID: projectID,
		Kind:      "workflow",
		Title:     title,
		Summary:   strings.TrimSpace(req.Summary),
	}
	if err := h.chats.Create(c.Request().Context(), ch); err != nil {
		h.logger.Error("create chat", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.JSON(http.StatusCreated, chatResponse{
		ID: ch.ID, ProjectID: ch.ProjectID, Kind: ch.Kind, Title: ch.Title, Summary: ch.Summary,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
}

func (h *ProjectsHandler) ListChats(c echo.Context) error {
	chats, err := h.chats.ListByProject(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.JSON(http.StatusOK, map[string]any{"chats": mapChatsToResponse(chats)})
}

func (h *ProjectsHandler) GetChat(c echo.Context) error {
	id := c.Param("id")
	ch, err := h.chats.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	payload := map[string]any{
		"chat": chatResponse{
			ID: ch.ID, ProjectID: ch.ProjectID, Kind: ch.Kind, Title: ch.Title, Summary: ch.Summary,
			LatestWorkflowVersionID: ch.LatestWorkflowVersionID,
			CreatedAt:               ch.CreatedAt, UpdatedAt: ch.UpdatedAt,
		},
	}
	// Include the current IR if one exists, so the client can render
	// the canvas without a second round-trip.
	if ch.LatestWorkflowVersionID != nil {
		if v, err := h.versions.Get(c.Request().Context(), *ch.LatestWorkflowVersionID); err == nil {
			payload["workflow"] = map[string]any{
				"id":          v.ID,
				"stage":       v.Stage,
				"ir":          json.RawMessage(v.IRJSON),
				"diagnostics": json.RawMessage(v.DiagnosticsJSON),
				"created_at":  v.CreatedAt,
			}
		}
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *ProjectsHandler) RenameChat(c echo.Context) error {
	var req chatRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	if err := h.chats.Rename(c.Request().Context(), c.Param("id"), strings.TrimSpace(req.Title)); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *ProjectsHandler) DeleteChat(c echo.Context) error {
	if err := h.chats.Delete(c.Request().Context(), c.Param("id")); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.NoContent(http.StatusNoContent)
}

// --- /messages ---

type messageRequest struct {
	Role string          `json:"role"` // defaults to "user"
	Body json.RawMessage `json:"body"` // free-shape envelope
}

type messageResponse struct {
	ID        string          `json:"id"`
	ChatID    string          `json:"chat_id"`
	Role      string          `json:"role"`
	Body      json.RawMessage `json:"body"`
	CreatedAt time.Time       `json:"created_at"`
}

// appendMessageResponse extends the simple message shape with the
// assistant reply + any new workflow version that extraction
// produced, so the UI can update its thread, canvas, and diagnostic
// surfaces from a single round-trip.
type appendMessageResponse struct {
	User      messageResponse  `json:"user"`
	Assistant *messageResponse `json:"assistant,omitempty"`
	Workflow  *workflowPayload `json:"workflow,omitempty"`
	Error     string           `json:"error,omitempty"`
}

type workflowPayload struct {
	ID          string          `json:"id"`
	Stage       string          `json:"stage"`
	IR          json.RawMessage `json:"ir"`
	Diagnostics json.RawMessage `json:"diagnostics"`
	CreatedAt   time.Time       `json:"created_at"`
}

// AppendMessage persists one user message and — when it's a text
// message from the user — kicks off extraction against the whole
// chat context (recent messages + attachment text). The assistant's
// reply + the resulting workflow version are written atomically so
// the UI's thread and canvas stay consistent.
func (h *ProjectsHandler) AppendMessage(c echo.Context) error {
	ctx := c.Request().Context()
	chatID := c.Param("id")
	chat, err := h.chats.Get(ctx, chatID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "chat_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	var req messageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "assistant" && role != "system" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_role"})
	}
	body := req.Body
	if len(body) == 0 {
		body = json.RawMessage(`{}`)
	}

	userMsg := repo.ChatMessage{
		ID:     makeID("m", role),
		ChatID: chatID,
		Role:   role,
		Body:   body,
	}
	if err := h.messages.Append(ctx, userMsg); err != nil {
		h.logger.Error("append message", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	// Bind any attachments referenced in the envelope to this
	// message so the UI can render chips inside the right bubble.
	for _, attID := range extractAttachmentIDs(body) {
		if err := h.attach.BindToMessage(ctx, attID, userMsg.ID); err != nil {
			h.logger.Warn("bind attachment", slog.String("id", attID), slog.String("err", err.Error()))
		}
	}

	resp := appendMessageResponse{
		User: messageResponse{
			ID: userMsg.ID, ChatID: userMsg.ChatID, Role: userMsg.Role, Body: userMsg.Body, CreatedAt: time.Now(),
		},
	}

	// If this is a user text message, run extraction. For any other
	// shape (assistant/system pushes, empty bodies) we just persist
	// and return — callers like the Copilot push assistant replies
	// directly and handle their own workflows.
	if role != "user" {
		return c.JSON(http.StatusCreated, resp)
	}
	text := extractTextFromBody(body)
	attIDs := extractAttachmentIDs(body)
	if text == "" && len(attIDs) == 0 {
		return c.JSON(http.StatusCreated, resp)
	}
	// If the user sent only attachments (no typed text), synthesize a
	// prompt so the extractor has an instruction. The attachments
	// themselves are already surfaced in chat_context.
	if text == "" {
		text = "Extract a workflow from the attached document(s)."
	}

	// Workflow-chat path. Onboarding interviews don't go through here
	// (they're a one-shot modal, not a per-turn chat) — see
	// POST /projects/:id/onboarding for that flow.
	//
	// Local Whisper/Tesseract may take a while on first cold call;
	// cap the round-trip at 4 minutes so a single slow turn doesn't
	// hang the UI forever — this is generous in practice (typical
	// turn is 60-110s on a 3B local model).
	ctxRun, cancel := context.WithTimeout(ctx, 240*time.Second)
	defer cancel()

	contextBlock, err := h.buildChatContext(ctxRun, chatID, userMsg.ID)
	if err != nil {
		h.logger.Warn("build chat context", slog.String("err", err.Error()))
	}

	agentIR, agentQuestions, agentErr := h.callAgentExtract(ctxRun, text, contextBlock)
	if agentErr != nil {
		// Persist a system note so the thread acknowledges the failure
		// and the user can retry. humanize() collapses verbose
		// provider-specific errors (Groq 429 blobs, etc.) into a
		// short one-liner the UI can show cleanly.
		niceErr := humanizeAgentError(agentErr.Error())
		failBody, _ := json.Marshal(map[string]any{"error": niceErr})
		_ = h.messages.Append(ctxRun, repo.ChatMessage{
			ID:     makeID("m", "system"),
			ChatID: chatID,
			Role:   "system",
			Body:   failBody,
		})
		resp.Error = niceErr
		return c.JSON(http.StatusOK, resp)
	}

	// Validate the returned IR. A schema-invalid response becomes a
	// system note; cross-ref warnings (unknown bindings) become
	// lowering-style diagnostics on the workflow.
	wf, schemaDiags, err := h.validator.ValidateWorkflowJSON(agentIR)
	_ = wf
	stage := "ready"
	diagsJSON := json.RawMessage("[]")
	if err != nil || len(schemaDiags) > 0 {
		// Still persist the IR so the user can see what the agent
		// produced; mark stage=drafting so the UI knows it's not
		// deploy-ready.
		stage = "drafting"
		if err != nil {
			schemaDiags = append(schemaDiags, ir.Diagnostic{Severity: "error", Message: err.Error()})
		}
		raw, _ := json.Marshal(schemaDiags)
		diagsJSON = raw
	}

	version := repo.WorkflowVersion{
		ID:              makeID("wv", chat.Title),
		ChatID:          chatID,
		IRJSON:          agentIR,
		Stage:           stage,
		DiagnosticsJSON: diagsJSON,
		SourceMessageID: &userMsg.ID,
	}
	if err := h.versions.Create(ctxRun, version); err != nil {
		h.logger.Error("persist workflow version", slog.String("err", err.Error()))
		resp.Error = "couldn't persist workflow"
		return c.JSON(http.StatusOK, resp)
	}
	_ = h.chats.SetLatestWorkflowVersion(ctxRun, chatID, version.ID)

	// Build the assistant reply. If the extractor raised clarifying
	// questions, lead with them — the text preamble changes tone so
	// the user sees "I still need to confirm X" rather than a
	// victory summary. When there are no questions, behave as before
	// with a plain extraction summary.
	assistantSummary := summarizeIR(agentIR, stage)
	if len(agentQuestions) > 0 {
		assistantSummary = leadInWithQuestions(agentIR, len(agentQuestions))
	}
	assistantPayload := map[string]any{
		"text":                assistantSummary,
		"workflow_version_id": version.ID,
	}
	if len(agentQuestions) > 0 {
		assistantPayload["questions"] = agentQuestions
	}
	assistantBody, _ := json.Marshal(assistantPayload)
	assistantMsg := repo.ChatMessage{
		ID:     makeID("m", "assistant"),
		ChatID: chatID,
		Role:   "assistant",
		Body:   assistantBody,
	}
	if err := h.messages.Append(ctxRun, assistantMsg); err != nil {
		h.logger.Warn("append assistant message", slog.String("err", err.Error()))
	}
	resp.Assistant = &messageResponse{
		ID: assistantMsg.ID, ChatID: assistantMsg.ChatID, Role: assistantMsg.Role, Body: assistantMsg.Body, CreatedAt: time.Now(),
	}
	resp.Workflow = &workflowPayload{
		ID: version.ID, Stage: stage, IR: agentIR, Diagnostics: diagsJSON, CreatedAt: time.Now(),
	}
	return c.JSON(http.StatusCreated, resp)
}

// extractTextFromBody pulls the "text" field out of the JSON body.
// Clients may post richer envelopes (attachments, tool markers); we
// only trigger extraction when there's plain text to work from.
func extractTextFromBody(body json.RawMessage) string {
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Text)
}

// extractAttachmentIDs pulls the optional "attachment_ids" array out
// of the message body. Empty / missing / malformed → no ids, which
// is fine — binding is cosmetic.
func extractAttachmentIDs(body json.RawMessage) []string {
	var parsed struct {
		AttachmentIDs []string `json:"attachment_ids"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil
	}
	return parsed.AttachmentIDs
}

// buildChatContext composes the chat_context string: recent messages
// (oldest-first, excluding the message we just inserted) + any
// attachment text already on the chat + the CURRENT IR (so
// refinement prompts like "rename the last node" operate on the
// authoritative structure rather than re-extracting from scratch).
func (h *ProjectsHandler) buildChatContext(ctx context.Context, chatID, excludeMessageID string) (string, error) {
	msgs, err := h.messages.Recent(ctx, chatID, 20)
	if err != nil {
		return "", err
	}
	atts, err := h.attach.ListByChat(ctx, chatID)
	if err != nil {
		return "", err
	}
	// Pull the chat's latest workflow so refinements modify what
	// the user is actually looking at. Without this, each message
	// triggers a fresh extraction and tweaks like "rename the last
	// task" come back with a random re-extraction instead.
	chat, _ := h.chats.Get(ctx, chatID)
	var currentIR json.RawMessage
	if chat != nil && chat.LatestWorkflowVersionID != nil {
		if v, verr := h.versions.Get(ctx, *chat.LatestWorkflowVersionID); verr == nil {
			currentIR = v.IRJSON
		}
	}

	var b strings.Builder
	if len(currentIR) > 0 {
		b.WriteString("CURRENT WORKFLOW (modify this unless the user asks for a completely new one):\n")
		b.Write(currentIR)
		b.WriteString("\n\n")
	}
	if len(atts) > 0 {
		b.WriteString("ATTACHMENTS:\n")
		for _, a := range atts {
			if len(a.TextContent) == 0 {
				continue
			}
			truncated := a.TextContent
			if len(truncated) > 1500 {
				truncated = truncated[:1500] + "…"
			}
			fmt.Fprintf(&b, "- %s (%s): %s\n", a.Filename, a.Kind, truncated)
		}
		b.WriteString("\n")
	}
	if len(msgs) > 0 {
		b.WriteString("PRIOR MESSAGES:\n")
		for _, m := range msgs {
			if m.ID == excludeMessageID {
				continue
			}
			text := extractTextFromBody(m.Body)
			if text == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s: %s\n", m.Role, text)
		}
	}
	return b.String(), nil
}

// ClarifyingQuestion is a single question the extractor wants the
// user to answer before it can raise an element's confidence. Kept
// minimal — the UI uses `text` for the bubble and `ir_ref` to
// optionally highlight the referenced node on the canvas.
type ClarifyingQuestion struct {
	ID    string `json:"id"`
	IRRef string `json:"ir_ref,omitempty"`
	Text  string `json:"text"`
}

// callAgentExtract calls the agent's /extract endpoint with an empty
// is_registry (the operator-tool repositioning no longer grounds in
// the IS) and the chat's composed context. Returns the IR plus any
// clarifying questions the extractor flagged for low-confidence
// elements.
func (h *ProjectsHandler) callAgentExtract(ctx context.Context, text, chatContext string) (json.RawMessage, []ClarifyingQuestion, error) {
	if h.agentURL == "" {
		return nil, nil, fmt.Errorf("agent URL not configured")
	}
	reqBody, _ := json.Marshal(map[string]any{
		"text":         text,
		"chat_context": chatContext,
		"is_registry":  map[string]any{"users": []any{}, "groups": []any{}, "systems": []any{}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.agentURL+"/extract", bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("agent unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("agent %d: %s", resp.StatusCode, truncateStr(string(raw), 300))
	}
	var env struct {
		IR        json.RawMessage      `json:"ir"`
		Questions []ClarifyingQuestion `json:"questions"`
		Error     string               `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, fmt.Errorf("decode agent response: %w", err)
	}
	if env.Error != "" {
		return nil, nil, fmt.Errorf("%s", env.Error)
	}
	if len(env.IR) == 0 {
		return nil, nil, fmt.Errorf("agent returned no IR")
	}
	return env.IR, env.Questions, nil
}

// summarizeIR produces a one-line assistant reply describing what
// extraction produced. Keeps the chat transcript readable without
// dumping the full JSON IR into a bubble.
func summarizeIR(rawIR json.RawMessage, stage string) string {
	var parsed struct {
		Tasks    []any `json:"tasks"`
		Gateways []any `json:"gateways"`
		Events   []any `json:"events"`
	}
	_ = json.Unmarshal(rawIR, &parsed)
	msg := fmt.Sprintf("Extracted %d task%s, %d gateway%s.", len(parsed.Tasks), plural(len(parsed.Tasks)), len(parsed.Gateways), plural(len(parsed.Gateways)))
	if stage == "drafting" {
		msg += " The IR needs a review — open the Clarify panel to resolve the flagged items before compiling."
	} else {
		msg += " The workflow is ready to compile or deploy."
	}
	return msg
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// leadInWithQuestions crafts a short preamble that replaces the
// usual "Extracted N tasks…" summary when the extractor raised
// clarifying questions. Keeps the tone conversational — the list
// of questions is rendered separately by the UI.
func leadInWithQuestions(rawIR json.RawMessage, nQ int) string {
	var parsed struct {
		Tasks []any `json:"tasks"`
	}
	_ = json.Unmarshal(rawIR, &parsed)
	if nQ == 1 {
		return fmt.Sprintf("I've drafted %d task%s, but I need one clarification before this is ready:", len(parsed.Tasks), plural(len(parsed.Tasks)))
	}
	return fmt.Sprintf("I've drafted %d task%s, but I need %d clarifications before this is ready:", len(parsed.Tasks), plural(len(parsed.Tasks)), nQ)
}

// ApproveWorkflow flips the chat's latest workflow_version stage
// from ready -> approved. Rejects drafting versions (the user must
// resolve low-confidence items first) and already-approved versions
// (idempotency, but surfaced as a 409 so the UI can distinguish).
func (h *ProjectsHandler) ApproveWorkflow(c echo.Context) error {
	ctx := c.Request().Context()
	chatID := c.Param("id")
	chat, err := h.chats.Get(ctx, chatID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "chat_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	if chat.LatestWorkflowVersionID == nil {
		return c.JSON(http.StatusConflict, map[string]string{
			"error":  "no_workflow",
			"detail": "this chat hasn't produced a workflow yet",
		})
	}
	v, err := h.versions.Get(ctx, *chat.LatestWorkflowVersionID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	switch v.Stage {
	case "approved":
		return c.JSON(http.StatusConflict, map[string]string{
			"error":  "already_approved",
			"detail": "the latest version is already approved",
		})
	case "drafting":
		return c.JSON(http.StatusConflict, map[string]string{
			"error":  "not_ready",
			"detail": "resolve the pending clarifications before approving",
		})
	case "ready":
		// allowed — fall through
	default:
		return c.JSON(http.StatusConflict, map[string]string{
			"error":  "unexpected_stage",
			"detail": v.Stage,
		})
	}
	if err := h.versions.SetStage(ctx, v.ID, "approved"); err != nil {
		h.logger.Error("approve workflow", slog.String("err", err.Error()))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"workflow_version_id": v.ID,
		"stage":               "approved",
	})
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// --- /onboarding ---

type onboardingRequest struct {
	QuestionIndex int      `json:"question_index"`
	Answer        string   `json:"answer"`
	MultiSelect   []string `json:"multi_select,omitempty"`
}

type onboardingResponse struct {
	Overview    json.RawMessage         `json:"overview,omitempty"`
	Questions   []DeterministicQuestion `json:"questions"`
	Complete    bool                    `json:"complete"`
	CurrentStep int                     `json:"current_step"`
	Error       string                  `json:"error,omitempty"`
}

type DeterministicQuestion struct {
	Index       int      `json:"index"`
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Type        string   `json:"type"` // text, single, multi
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required"`
	Placeholder string   `json:"placeholder,omitempty"`
}

var deterministicQuestions = []DeterministicQuestion{
	{
		Index:       0,
		ID:          "name",
		Text:        "What is the organisation called?",
		Type:        "text",
		Required:    true,
		Placeholder: "e.g. Acme Corp",
	},
	{
		Index:    1,
		ID:       "industry",
		Text:     "What industry are you in?",
		Type:     "single",
		Required: true,
		Options:  []string{"Healthcare", "Finance", "Logistics", "Manufacturing", "Retail", "Education", "Energy", "Telecommunications", "Government", "Other"},
	},
	{
		Index:    2,
		ID:       "size",
		Text:     "How many employees?",
		Type:     "single",
		Required: true,
		Options:  []string{"1-10", "10-50", "50-200", "200-1000", "1000+"},
	},
	{
		Index:    3,
		ID:       "regions",
		Text:     "Where do you operate?",
		Type:     "multi",
		Required: true,
		Options:  []string{"Algeria", "France", "Tunisia", "Morocco", "Egypt", "Libya", "Mauritania", "Other"},
	},
	{
		Index:    4,
		ID:       "systems",
		Text:     "What business systems do you use?",
		Type:     "multi",
		Required: false,
		Options:  []string{"ERP", "CRM", "Accounting", "HRM", "Supply Chain", "None", "Other"},
	},
	{
		Index:       5,
		ID:          "processes",
		Text:        "What are your main business processes?",
		Type:        "multi",
		Required:    false,
		Options:     []string{"Order Management", "Invoicing", "Inventory", "Procurement", "HR", "Customer Support", "Finance", "Marketing", "Sales", "Other"},
	},
	{
		Index:    6,
		ID:       "compliance",
		Text:     "Any compliance requirements?",
		Type:     "multi",
		Required: false,
		Options:  []string{"GDPR", "ISO27001", "SOC2", "HIPAA", "PCI-DSS", "None", "Other"},
	},
	{
		Index:    7,
		ID:       "languages",
		Text:     "What languages does your organisation use?",
		Type:     "multi",
		Required: true,
		Options:  []string{"French", "Arabic", "English", "Spanish", "Other"},
	},
}

// GetOnboardingQuestions returns the deterministic questions for onboarding.
// This endpoint doesn't require a project ID - it's used before project creation.
func (h *ProjectsHandler) GetOnboardingQuestions(c echo.Context) error {
	return c.JSON(http.StatusOK, onboardingResponse{
		Questions:   deterministicQuestions,
		Complete:    false,
		CurrentStep: 0,
	})
}

// OnboardProject handles the deterministic onboarding wizard.
// It returns fixed questions and stores answers in projects.overview_json.
// If projectID is empty, it returns questions without saving (for initial load).
func (h *ProjectsHandler) OnboardProject(c echo.Context) error {
	ctx := c.Request().Context()
	projectID := c.Param("id")

	var req onboardingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_json"})
	}

	// If question_index is -1, just return questions (initial load, no project yet)
	if req.QuestionIndex == -1 {
		return c.JSON(http.StatusOK, onboardingResponse{
			Questions:   deterministicQuestions,
			Complete:    false,
			CurrentStep: 0,
		})
	}

	// For actual answers, project must exist
	if projectID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "project_required"})
	}

	// Verify project exists
	_, err := h.projects.Get(ctx, projectID)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "project_not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}

	// Get current overview from the project
	overview, _ := h.projects.GetOverview(ctx, projectID)
	var overviewMap map[string]any
	if overview != nil && len(overview) > 0 && string(overview) != "null" {
		_ = json.Unmarshal(overview, &overviewMap)
	}
	if overviewMap == nil {
		overviewMap = make(map[string]any)
	}

	// If we have an answer, save it
	if req.QuestionIndex >= 0 && req.QuestionIndex < len(deterministicQuestions) {
		q := deterministicQuestions[req.QuestionIndex]
		answer := strings.TrimSpace(req.Answer)
		multiSelect := req.MultiSelect

		if q.Type == "multi" && len(multiSelect) > 0 {
			overviewMap[q.ID] = multiSelect
		} else if answer != "" {
			overviewMap[q.ID] = answer
		}

		// Save updated overview
		updatedOverview, _ := json.Marshal(overviewMap)
		_ = h.projects.SetOverview(ctx, projectID, updatedOverview)
	}

	// Determine current step and if complete
	currentStep := req.QuestionIndex + 1
	complete := currentStep >= len(deterministicQuestions)

	// Check if any required fields are still missing
	if !complete {
		for i := currentStep; i < len(deterministicQuestions); i++ {
			q := deterministicQuestions[i]
			if q.Required && overviewMap[q.ID] == nil {
				// Found first unanswered required question
				break
			}
			currentStep = i + 1
		}
		complete = currentStep >= len(deterministicQuestions)
	}

	return c.JSON(http.StatusOK, onboardingResponse{
		Overview:    func() json.RawMessage { b, _ := json.Marshal(overviewMap); return b }(),
		Questions:   deterministicQuestions,
		Complete:    complete,
		CurrentStep: currentStep,
	})
}

// callAgentInterview proxies to the agent's /interview endpoint.
func (h *ProjectsHandler) callAgentInterview(ctx context.Context, text, chatContext string, priorOverview json.RawMessage) (json.RawMessage, []ClarifyingQuestion, error) {
	if h.agentURL == "" {
		return nil, nil, fmt.Errorf("agent URL not configured")
	}

	var priorMap map[string]any
	if len(priorOverview) > 0 && string(priorOverview) != "null" {
		_ = json.Unmarshal(priorOverview, &priorMap)
	}

	reqBody, _ := json.Marshal(map[string]any{
		"text":           text,
		"chat_context":   chatContext,
		"prior_overview": priorMap,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.agentURL+"/interview", bytes.NewReader(reqBody))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("content-type", "application/json")
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("agent unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, fmt.Errorf("agent %d: %s", resp.StatusCode, truncateStr(string(raw), 300))
	}
	var env struct {
		Overview  json.RawMessage      `json:"overview"`
		Questions []ClarifyingQuestion `json:"questions"`
		Error     string               `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, nil, fmt.Errorf("decode agent response: %w", err)
	}
	if env.Error != "" {
		return nil, nil, fmt.Errorf("%s", env.Error)
	}
	return env.Overview, env.Questions, nil
}

func (h *ProjectsHandler) ListMessages(c echo.Context) error {
	msgs, err := h.messages.ListByChat(c.Request().Context(), c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db_error"})
	}
	out := make([]messageResponse, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, messageResponse{
			ID: m.ID, ChatID: m.ChatID, Role: m.Role, Body: m.Body, CreatedAt: m.CreatedAt,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"messages": out})
}

// --- helpers ---

// makeID produces a short human-readable id of the form
// `<prefix>_<slug>_<rand>`. Slug is derived from the most relevant
// user-visible string (name/title/role) so ids remain debuggable in
// SQL inspections, and the rand suffix guarantees uniqueness.
var idSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

func makeID(prefix, seed string) string {
	slug := strings.ToLower(strings.TrimSpace(seed))
	slug = idSlugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 16 {
		slug = slug[:16]
	}
	if slug == "" {
		slug = "x"
	}
	var b [4]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + slug + "_" + hex.EncodeToString(b[:])
}

// humanizeAgentError collapses provider-specific failure blobs
// (Gemini 429 JSON, network timeouts, etc.) into short UI-friendly
// messages. Matches on substrings because the upstream wire format
// changes without warning; we fall back to a truncated raw message
// when we don't recognize the shape.
func humanizeAgentError(raw string) string {
	lower := strings.ToLower(raw)
	switch {
	// --- Ollama (local LLM) ---
	case strings.Contains(lower, "connection refused") && strings.Contains(lower, "11434"),
		strings.Contains(lower, "host.docker.internal:11434"):
		return "Can't reach Ollama on the host. Start it with `ollama serve`, or pull a model with `ollama run qwen3.5:9b`."
	case strings.Contains(lower, "model") && strings.Contains(lower, "not found") && strings.Contains(lower, "ollama"),
		strings.Contains(lower, "pull the model first"):
		return "The Ollama model isn't installed locally. Run `ollama pull qwen3.5:9b` (or whatever AGENT_EXTRACTOR_MODEL is set to)."
	// --- Gemini (cloud) ---
	case strings.Contains(lower, "resource_exhausted"), strings.Contains(lower, "429"), strings.Contains(lower, "quota"):
		retryHint := ""
		if idx := strings.Index(raw, "retryDelay"); idx > 0 {
			tail := raw[idx:]
			if q1 := strings.Index(tail, "'"); q1 > 0 {
				rest := tail[q1+1:]
				if q2 := strings.Index(rest, "'"); q2 > 0 {
					retryHint = " Retry in ~" + rest[:q2] + "."
				}
			}
		}
		return "Groq's quota may be exceeded." + retryHint + " Check your GROQ_API_KEY or try again later."
	case strings.Contains(lower, "context deadline exceeded"), strings.Contains(lower, "timeout"):
		return "The extractor took too long. Try again — if the request was very large, shorten it. Local Ollama models are slower on first cold call; a retry often succeeds."
	case strings.Contains(lower, "no address associated with hostname"), strings.Contains(lower, "no such host"):
		return "The agent couldn't resolve the LLM host. Check the agent container's network + OLLAMA_BASE_URL."
	case strings.Contains(lower, "unauthorized"), strings.Contains(lower, "401"):
		return "The LLM rejected the API key. Rotate GOOGLE_API_KEY / ANTHROPIC_API_KEY on the agent, or switch AGENT_EXTRACTOR_PROVIDER to ollama."
	case strings.Contains(lower, "agent unreachable"):
		return "The agent service is offline. Check `docker compose ps agent`."
	}
	if len(raw) > 200 {
		return raw[:200] + "…"
	}
	return raw
}

// Workflow Versioning Handlers

type WorkflowVersionResponse struct {
	ID              string          `json:"id"`
	ChatID          string          `json:"chat_id"`
	Stage           string          `json:"stage"`
	IR              json.RawMessage `json:"ir"`
	Diagnostics     json.RawMessage `json:"diagnostics,omitempty"`
	SourceMessageID *string        `json:"source_message_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

func (h *ProjectsHandler) ListWorkflowVersions(c echo.Context) error {
	chatID := c.Param("id")
	if chatID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "chat id required"})
	}

	versions, err := h.versions.ListByChat(c.Request().Context(), chatID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	out := make([]WorkflowVersionResponse, len(versions))
	for i, v := range versions {
		out[i] = WorkflowVersionResponse{
			ID:              v.ID,
			ChatID:          v.ChatID,
			Stage:           v.Stage,
			IR:              v.IRJSON,
			Diagnostics:     v.DiagnosticsJSON,
			SourceMessageID: v.SourceMessageID,
			CreatedAt:       v.CreatedAt,
		}
	}
	return c.JSON(http.StatusOK, map[string][]WorkflowVersionResponse{"versions": out})
}

func (h *ProjectsHandler) GetWorkflowVersion(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "version id required"})
	}

	v, err := h.versions.Get(c.Request().Context(), id)
	if err != nil {
		if err == repo.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "version not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, WorkflowVersionResponse{
		ID:              v.ID,
		ChatID:          v.ChatID,
		Stage:           v.Stage,
		IR:              v.IRJSON,
		Diagnostics:     v.DiagnosticsJSON,
		SourceMessageID: v.SourceMessageID,
		CreatedAt:       v.CreatedAt,
	})
}

type ForkWorkflowRequest struct {
	VersionID    string `json:"version_id,omitempty"`
	TargetChatID string `json:"target_chat_id,omitempty"`
	Title        string `json:"title,omitempty"`
}

func (h *ProjectsHandler) ForkWorkflow(c echo.Context) error {
	chatID := c.Param("id")
	if chatID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "chat id required"})
	}

	var req ForkWorkflowRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Generate IDs
	sourceVersionID := req.VersionID
	if sourceVersionID == "" {
		sourceVersionID = c.QueryParam("version_id")
	}
	if sourceVersionID == "" {
		// Use latest version if not specified
		v, err := h.versions.LatestForChat(c.Request().Context(), chatID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "no workflow version found"})
		}
		sourceVersionID = v.ID
	}

	var b [8]byte
	_, _ = rand.Read(b[:])
	newVersionID := "wv_" + hex.EncodeToString(b[:])

	targetChatID, err := h.versions.Fork(c.Request().Context(), sourceVersionID, req.TargetChatID, newVersionID, req.Title)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "fork failed: " + err.Error()})
	}

	// Get the new chat and version to return a full response
	newChat, _ := h.chats.Get(c.Request().Context(), targetChatID)
	newVersion, _ := h.versions.Get(c.Request().Context(), newVersionID)

	return c.JSON(http.StatusOK, map[string]any{
		"chat":    mapChatsToResponse([]repo.Chat{*newChat})[0],
		"version": newVersion,
	})
}

func (h *ProjectsHandler) RestoreWorkflowVersion(c echo.Context) error {
	versionID := c.Param("id")
	if versionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "version id required"})
	}

	var b [8]byte
	_, _ = rand.Read(b[:])
	newVersionID := "wv_" + hex.EncodeToString(b[:])

	err := h.versions.Restore(c.Request().Context(), versionID, newVersionID)
	if err != nil {
		if err == repo.ErrNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "version not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "restore failed: " + err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"version_id": newVersionID})
}

type DiffRequest struct {
	OtherVersionID string `json:"other_version_id"`
}

func (h *ProjectsHandler) DiffWorkflowVersions(c echo.Context) error {
	versionID1 := c.Param("id")
	var req DiffRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	if versionID1 == "" || req.OtherVersionID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "both version1 and other_version_id required"})
	}

	v1, err := h.versions.Get(c.Request().Context(), versionID1)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "version1 not found"})
	}
	v2, err := h.versions.Get(c.Request().Context(), req.OtherVersionID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "version2 not found"})
	}

	diff := computeWorkflowDiff(v1.IRJSON, v2.IRJSON)
	return c.JSON(http.StatusOK, diff)
}

type DiffResponse struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// Simple diff computation - compares workflow structure
func computeWorkflowDiff(ir1, ir2 json.RawMessage) DiffResponse {
	var w1, w2 map[string]interface{}
	json.Unmarshal(ir1, &w1)
	json.Unmarshal(ir2, &w2)

	diff := DiffResponse{Added: []string{}, Removed: []string{}, Changed: []string{}}

	// Compare top-level keys
	keys1 := make(map[string]bool)
	keys2 := make(map[string]bool)

	if w1 != nil {
		for k := range w1 {
			keys1[k] = true
		}
	}
	if w2 != nil {
		for k := range w2 {
			keys2[k] = true
		}
	}

	for k := range keys2 {
		if !keys1[k] {
			diff.Added = append(diff.Added, k)
		}
	}
	for k := range keys1 {
		if !keys2[k] {
			diff.Removed = append(diff.Removed, k)
		}
	}

	// Check if any common keys have different values
	for k := range keys1 {
		if keys2[k] {
			v1, _ := json.Marshal(w1[k])
			v2, _ := json.Marshal(w2[k])
			if string(v1) != string(v2) {
				diff.Changed = append(diff.Changed, k)
			}
		}
	}

	return diff
}
