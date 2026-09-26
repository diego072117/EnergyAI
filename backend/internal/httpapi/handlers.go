package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"energyai/internal/ingest"
	"energyai/internal/service"
)

func (a *api) health(w http.ResponseWriter, r *http.Request) {
	if a.Health != nil {
		if err := a.Health(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "degraded", "database": "unreachable"})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *api) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "correo y contraseña son obligatorios")
		return
	}
	res, err := a.Auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(ctxKey{}).(*service.Claims)
	writeJSON(w, http.StatusOK, map[string]string{"email": claims.Email, "name": claims.Name})
}

func (a *api) dashboard(w http.ResponseWriter, r *http.Request) {
	s, err := a.Dashboard.Summary(r.Context())
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (a *api) listMeters(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	items, err := a.Meters.List(r.Context(), service.MeterFilter{
		Status: q.Get("status"), Query: q.Get("q"), Sort: q.Get("sort"), Order: q.Get("order"),
	})
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (a *api) getMeter(w http.ResponseWriter, r *http.Request) {
	d, err := a.Meters.Get(r.Context(), chi.URLParam(r, "meterId"))
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (a *api) meterReadings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from, err := optionalTime(q.Get("from"))
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	to, err := optionalTime(q.Get("to"))
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	s, err := a.Meters.Readings(r.Context(), chi.URLParam(r, "meterId"),
		service.ReadingsParams{From: from, To: to, Resolution: q.Get("resolution")})
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func optionalTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := ingest.ParseTime(s)
	if err != nil {
		return nil, fmt.Errorf("%w: fecha inválida %q (use RFC3339 o YYYY-MM-DD HH:MM)", service.ErrInvalidInput, s)
	}
	return &t, nil
}

func (a *api) listAnomalies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := a.Anomalies.List(r.Context(), service.AnomalyQuery{
		AnalysisID: q.Get("analysis_id"), MeterID: q.Get("meter_id"),
		Type: q.Get("type"), Severity: q.Get("severity"), Status: q.Get("status"),
	})
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func anomalyID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: id de anomalía inválido", service.ErrInvalidInput)
	}
	return id, nil
}

func (a *api) getAnomaly(w http.ResponseWriter, r *http.Request) {
	id, err := anomalyID(r)
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	d, err := a.Anomalies.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

type updateAnomalyRequest struct {
	Status string  `json:"status"`
	Note   *string `json:"note"`
}

func (a *api) updateAnomaly(w http.ResponseWriter, r *http.Request) {
	id, err := anomalyID(r)
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	var req updateAnomalyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	updated, err := a.Anomalies.UpdateStatus(r.Context(), id, req.Status, req.Note)
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (a *api) analyze(w http.ResponseWriter, r *http.Request) {
	run, err := a.Runner.Start(r.Context())
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	w.Header().Set("Location", "/api/v1/ai/analysis/"+run.ID)
	writeJSON(w, http.StatusAccepted, run)
}

func (a *api) getAnalysis(w http.ResponseWriter, r *http.Request) {
	run, err := a.Runner.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (a *api) latestAnalysis(w http.ResponseWriter, r *http.Request) {
	run, err := a.Runner.Latest(r.Context())
	if err != nil {
		writeServiceError(w, r, a.Log, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (a *api) aiStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.AIStatus.Status(r.Context()))
}
