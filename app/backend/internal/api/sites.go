package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"web-monitor/internal/auth"
	"web-monitor/internal/sites"
	"web-monitor/internal/store"
)

// CreateSiteRequest defines the expected JSON payload for adding a site.
type CreateSiteRequest struct {
	URL               string   `json:"url"`
	Label             *string  `json:"label,omitempty"`
	CheckIntervalSec  int      `json:"check_interval_sec,omitempty"`
	TimeoutSec        int      `json:"timeout_sec,omitempty"`
	FailureThreshold  int      `json:"failure_threshold,omitempty"`
	RetryCount        *int     `json:"retry_count,omitempty"`
	ExpectedStatusMin int      `json:"expected_status_min,omitempty"`
	ExpectedStatusMax int      `json:"expected_status_max,omitempty"`
	Tags              []string `json:"tags,omitempty"`
}

// UpdateSiteRequest defines the expected JSON payload for updating a site.
type UpdateSiteRequest struct {
	Label             *string   `json:"label,omitempty"`
	CheckIntervalSec  *int      `json:"check_interval_sec,omitempty"`
	TimeoutSec        *int      `json:"timeout_sec,omitempty"`
	FailureThreshold  *int      `json:"failure_threshold,omitempty"`
	RetryCount        *int      `json:"retry_count,omitempty"`
	ExpectedStatusMin *int      `json:"expected_status_min,omitempty"`
	ExpectedStatusMax *int      `json:"expected_status_max,omitempty"`
	Tags              *[]string `json:"tags,omitempty"`
}

// SiteResponse wraps a single site in standard response format.
type SiteResponse struct {
	Site *store.SiteWithTags `json:"site"`
}

// SitesListResponse wraps a list of sites in standard response format.
type SitesListResponse struct {
	Sites []*store.SiteWithTags `json:"sites"`
}

func (client *APIClient) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var req CreateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	site, err := client.sitesService.CreateSite(r.Context(), userID, sites.CreateSiteInput{
		URL:               req.URL,
		Label:             req.Label,
		CheckIntervalSec:  req.CheckIntervalSec,
		TimeoutSec:        req.TimeoutSec,
		FailureThreshold:  req.FailureThreshold,
		RetryCount:        req.RetryCount,
		ExpectedStatusMin: req.ExpectedStatusMin,
		ExpectedStatusMax: req.ExpectedStatusMax,
		Tags:              req.Tags,
	})
	if err != nil {
		switch {
		case errors.Is(err, sites.ErrInvalidURL):
			writeError(w, http.StatusBadRequest, "Invalid URL: must be a valid http or https URL with a hostname")
		case errors.Is(err, sites.ErrInvalidCheckInterval):
			writeError(w, http.StatusBadRequest, "Check interval must be between 10 and 86400 seconds")
		case errors.Is(err, sites.ErrInvalidTimeout):
			writeError(w, http.StatusBadRequest, "Timeout must be between 1 and 60 seconds")
		case errors.Is(err, sites.ErrInvalidFailureThreshold):
			writeError(w, http.StatusBadRequest, "Failure threshold must be between 1 and 10")
		case errors.Is(err, sites.ErrInvalidRetryCount):
			writeError(w, http.StatusBadRequest, "Retry count must be between 0 and 5")
		case errors.Is(err, sites.ErrInvalidExpectedStatus):
			writeError(w, http.StatusBadRequest, "Expected status range must be between 100 and 599, with min <= max")
		case errors.Is(err, store.ErrDuplicateSite):
			writeError(w, http.StatusConflict, "A site with this URL is already being monitored")
		default:
			writeError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, SiteResponse{Site: site})
}

func (client *APIClient) handleListSites(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	tagFilter := r.URL.Query().Get("tag")
	siteList, err := client.sitesService.ListSites(r.Context(), userID, tagFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, SitesListResponse{Sites: siteList})
}

func (client *APIClient) handleGetSite(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := r.PathValue("id")
	siteID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid site ID")
		return
	}

	site, err := client.sitesService.GetSite(r.Context(), userID, siteID)
	if err != nil {
		if errors.Is(err, store.ErrSiteNotFound) {
			writeError(w, http.StatusNotFound, "Site not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, SiteResponse{Site: site})
}

func (client *APIClient) handleUpdateSite(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := r.PathValue("id")
	siteID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid site ID")
		return
	}

	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}

	var req UpdateSiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	updated, err := client.sitesService.UpdateSite(r.Context(), userID, siteID, store.UpdateSiteInput{
		Label:             req.Label,
		CheckIntervalSec:  req.CheckIntervalSec,
		TimeoutSec:        req.TimeoutSec,
		FailureThreshold:  req.FailureThreshold,
		RetryCount:        req.RetryCount,
		ExpectedStatusMin: req.ExpectedStatusMin,
		ExpectedStatusMax: req.ExpectedStatusMax,
		Tags:              req.Tags,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrSiteNotFound):
			writeError(w, http.StatusNotFound, "Site not found")
		case errors.Is(err, sites.ErrInvalidCheckInterval):
			writeError(w, http.StatusBadRequest, "Check interval must be between 10 and 86400 seconds")
		case errors.Is(err, sites.ErrInvalidTimeout):
			writeError(w, http.StatusBadRequest, "Timeout must be between 1 and 60 seconds")
		case errors.Is(err, sites.ErrInvalidFailureThreshold):
			writeError(w, http.StatusBadRequest, "Failure threshold must be between 1 and 10")
		case errors.Is(err, sites.ErrInvalidRetryCount):
			writeError(w, http.StatusBadRequest, "Retry count must be between 0 and 5")
		case errors.Is(err, sites.ErrInvalidExpectedStatus):
			writeError(w, http.StatusBadRequest, "Expected status range must be between 100 and 599, with min <= max")
		default:
			writeError(w, http.StatusInternalServerError, "Internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, SiteResponse{Site: updated})
}

func (client *APIClient) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	idStr := r.PathValue("id")
	siteID, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid site ID")
		return
	}

	if err := client.sitesService.DeleteSite(r.Context(), userID, siteID); err != nil {
		if errors.Is(err, store.ErrSiteNotFound) {
			writeError(w, http.StatusNotFound, "Site not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "Site deleted successfully"})
}
