package web

import (
	"io/fs"
	"net/http"
	"time"

	"kilncurve-release/internal/application"
	"kilncurve-release/internal/domain"
)

type Handler struct {
	service *application.Service
	static  http.Handler
}

func NewHandler(service *application.Service) http.Handler {
	sub, _ := fs.Sub(assets, "static")
	h := &Handler{service: service, static: http.FileServer(http.FS(sub))}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.HandleIndex)
	mux.HandleFunc("GET /api/health", h.HandleHealth)
	mux.HandleFunc("GET /api/projects", h.HandleListProjects)
	mux.HandleFunc("POST /api/projects", h.HandleCreateProject)
	mux.HandleFunc("GET /api/projects/{projectId}", h.HandleGetProject)
	mux.HandleFunc("PATCH /api/projects/{projectId}/boundaries", h.HandleReviseBoundaries)
	mux.HandleFunc("POST /api/projects/{projectId}/curve/validate", h.HandleValidateCurve)
	mux.HandleFunc("GET /api/projects/{projectId}/revisions/compare", h.HandleCompareRevisions)
	mux.HandleFunc("POST /api/projects/{projectId}/revisions", h.HandleCreateRevision)
	mux.HandleFunc("PUT /api/projects/{projectId}/revisions/{revisionId}", h.HandleEditRevision)
	mux.HandleFunc("POST /api/projects/{projectId}/revisions/{revisionId}/derive", h.HandleDeriveRevision)
	mux.HandleFunc("POST /api/projects/{projectId}/revisions/{revisionId}/freeze", h.HandleFreezeRevision)
	mux.HandleFunc("POST /api/projects/{projectId}/trial-runs", h.HandleTrialRun)
	mux.HandleFunc("POST /api/projects/{projectId}/trial-runs/drafts", h.HandleStartTrialRun)
	mux.HandleFunc("PUT /api/projects/{projectId}/trial-runs/{runId}/evidence", h.HandleSaveTrialEvidence)
	mux.HandleFunc("POST /api/projects/{projectId}/trial-runs/{runId}/complete", h.HandleCompleteTrialRun)
	mux.HandleFunc("POST /api/projects/{projectId}/deviations/{deviationId}/correct", h.HandleCorrectDeviation)
	mux.HandleFunc("POST /api/projects/{projectId}/deviation-batches", h.HandleCreateCorrectionBatch)
	mux.HandleFunc("POST /api/projects/{projectId}/review", h.HandleReview)
	mux.HandleFunc("GET /api/process-cards/{cardId}/verify", h.HandleVerifyCard)
	return requestMiddleware(securityHeaders(mux))
}
func (h *Handler) HandleReviseBoundaries(w http.ResponseWriter, r *http.Request) {
	var c application.ReviseBoundariesCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.ReviseBoundaries(r.PathValue("projectId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		b, err := assets.ReadFile("static/index.html")
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
		return
	}
	h.static.ServeHTTP(w, r)
}
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.Diagnostics()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC(), "persistence": stats})
}
func (h *Handler) HandleListProjects(w http.ResponseWriter, r *http.Request) {
	v, err := h.service.ListProjects(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": v})
}
func (h *Handler) HandleCreateProject(w http.ResponseWriter, r *http.Request) {
	var c application.CreateProjectCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.CreateProject(c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h *Handler) HandleGetProject(w http.ResponseWriter, r *http.Request) {
	v, err := h.service.GetProject(r.PathValue("projectId"), r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) HandleValidateCurve(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Segments []domain.CurveSegment `json:"segments"`
	}
	if err := decodeStrict(w, r, &body); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.ValidateCurve(r.PathValue("projectId"), body.Segments, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": v})
}
func (h *Handler) HandleCompareRevisions(w http.ResponseWriter, r *http.Request) {
	v, err := h.service.CompareRevisions(r.PathValue("projectId"), r.URL.Query().Get("baselineRevisionId"), r.URL.Query().Get("comparisonRevisionId"), r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) HandleCreateRevision(w http.ResponseWriter, r *http.Request) {
	var c application.RevisionCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.CreateRevision(r.PathValue("projectId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h *Handler) HandleEditRevision(w http.ResponseWriter, r *http.Request) {
	var c application.EditRevisionCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.EditRevision(r.PathValue("projectId"), r.PathValue("revisionId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) HandleDeriveRevision(w http.ResponseWriter, r *http.Request) {
	var c application.DeriveRevisionCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.DeriveRevision(r.PathValue("projectId"), r.PathValue("revisionId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h *Handler) HandleFreezeRevision(w http.ResponseWriter, r *http.Request) {
	var c application.FreezeCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.FreezeRevision(r.PathValue("projectId"), r.PathValue("revisionId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) HandleTrialRun(w http.ResponseWriter, r *http.Request) {
	var c application.TrialRunCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.RecordAndEvaluateRun(r.PathValue("projectId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h *Handler) HandleStartTrialRun(w http.ResponseWriter, r *http.Request) {
	var c application.StartTrialRunCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.StartTrialRun(r.PathValue("projectId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h *Handler) HandleSaveTrialEvidence(w http.ResponseWriter, r *http.Request) {
	var c application.SaveTrialEvidenceCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.SaveTrialEvidence(r.PathValue("projectId"), r.PathValue("runId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) HandleCompleteTrialRun(w http.ResponseWriter, r *http.Request) {
	var c application.CompleteTrialRunCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.CompleteTrialRun(r.PathValue("projectId"), r.PathValue("runId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) HandleCorrectDeviation(w http.ResponseWriter, r *http.Request) {
	var c application.CorrectionCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.CorrectDeviation(r.PathValue("projectId"), r.PathValue("deviationId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
func (h *Handler) HandleCreateCorrectionBatch(w http.ResponseWriter, r *http.Request) {
	var c application.CorrectionBatchCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.CreateCorrectionBatch(r.PathValue("projectId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}
func (h *Handler) HandleReview(w http.ResponseWriter, r *http.Request) {
	var c application.ReviewCommand
	if err := decodeStrict(w, r, &c); err != nil {
		writeError(w, err)
		return
	}
	v, err := h.service.ReviewProject(r.PathValue("projectId"), c, r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"processCard": v})
}
func (h *Handler) HandleVerifyCard(w http.ResponseWriter, r *http.Request) {
	card, valid, err := h.service.VerifyCard(r.PathValue("cardId"), r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"card": card, "valid": valid})
}
