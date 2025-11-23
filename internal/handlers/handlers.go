package handlers

import (
	"encoding/json"
	"net/http"
	"pr-reviewer-service/internal/models"
	"pr-reviewer-service/internal/service"
)

type Handlers struct {
	service *service.Service
}

func NewHandlers(service *service.Service) *Handlers {
	return &Handlers{service: service}
}

func (h *Handlers) AddTeam(w http.ResponseWriter, r *http.Request) {
	var team models.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	if err := h.service.CreateTeam(r.Context(), &team); err != nil {
		if err.Error() == "team already exists" {
			writeError(w, http.StatusBadRequest, "TEAM_EXISTS", "team_name already exists")
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"team": team})
}

func (h *Handlers) GetTeam(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team_name")
	if teamName == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PARAMETER", "team_name parameter is required")
		return
	}

	team, err := h.service.GetTeam(r.Context(), teamName)
	if err != nil {
		if err.Error() == "team not found" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, team)
}

func (h *Handlers) SetUserActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		IsActive bool   `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	user, err := h.service.UpdateUserActivity(r.Context(), req.UserID, req.IsActive)
	if err != nil {
		if err.Error() == "user not found" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"user": user})
}

func (h *Handlers) CreatePullRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID   string `json:"pull_request_id"`
		PullRequestName string `json:"pull_request_name"`
		AuthorID        string `json:"author_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	pr := &models.PullRequest{
		PullRequestID:   req.PullRequestID,
		PullRequestName: req.PullRequestName,
		AuthorID:        req.AuthorID,
	}

	createdPR, err := h.service.CreatePullRequest(r.Context(), pr)
	if err != nil {
		switch err.Error() {
		case "author not found":
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		case "PR id already exists":
			writeError(w, http.StatusConflict, "PR_EXISTS", "PR id already exists")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"pr": createdPR})
}

func (h *Handlers) MergePullRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	pr, err := h.service.MergePullRequest(r.Context(), req.PullRequestID)
	if err != nil {
		if err.Error() == "PR not found" {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		} else {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"pr": pr})
}

func (h *Handlers) ReassignReviewer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PullRequestID string `json:"pull_request_id"`
		OldUserID     string `json:"old_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	pr, newUserID, err := h.service.ReassignReviewer(r.Context(), req.PullRequestID, req.OldUserID)
	if err != nil {
		switch err.Error() {
		case "PR not found":
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		case "cannot reassign on merged PR":
			writeError(w, http.StatusConflict, "PR_MERGED", "cannot reassign on merged PR")
		case "reviewer is not assigned to this PR":
			writeError(w, http.StatusConflict, "NOT_ASSIGNED", "reviewer is not assigned to this PR")
		case "old reviewer not found":
			writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		case "no active replacement candidate in team":
			writeError(w, http.StatusConflict, "NO_CANDIDATE", "no active replacement candidate in team")
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pr":          pr,
		"replaced_by": newUserID,
	})
}

func (h *Handlers) GetUserReviewPullRequests(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "MISSING_PARAMETER", "user_id parameter is required")
		return
	}

	prs, err := h.service.GetUserReviewPullRequests(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":       userID,
		"pull_requests": prs,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, status int, code string, message ...string) {
	errorMsg := code
	if len(message) > 0 {
		errorMsg = message[0]
	}

	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": errorMsg,
		},
	})
}
