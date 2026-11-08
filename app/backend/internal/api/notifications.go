package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"web-monitor/internal/auth"
	"web-monitor/internal/store"
)

// CreateChannelRequest defines the payload for creating a notification channel.
type CreateChannelRequest struct {
	SiteID    *uuid.UUID        `json:"site_id,omitempty"`
	Type      store.ChannelType `json:"type"`
	Target    string            `json:"target"`
	IsEnabled *bool             `json:"is_enabled,omitempty"` // defaults to true
}

// UpdateChannelRequest defines the payload for updating a notification channel.
type UpdateChannelRequest struct {
	Target    *string `json:"target,omitempty"`
	IsEnabled *bool   `json:"is_enabled,omitempty"`
}

// ChannelResponse wraps a single notification channel.
type ChannelResponse struct {
	Channel *store.NotificationChannel `json:"channel"`
}

// ChannelsListResponse wraps a list of notification channels.
type ChannelsListResponse struct {
	Channels []*store.NotificationChannel `json:"channels"`
}

// EventsListResponse wraps a paginated list of notification events.
type EventsListResponse struct {
	Events []*store.NotificationEvent `json:"events"`
	Limit  int                        `json:"limit"`
	Offset int                        `json:"offset"`
}

func validateTarget(cType store.ChannelType, target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("target cannot be empty")
	}
	switch cType {
	case store.ChannelTypeEmail:
		if _, err := auth.ValidateAndNormalizeEmail(target); err != nil {
			return err
		}
	case store.ChannelTypeWebhook:
		parsed, err := url.ParseRequestURI(target)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errors.New("invalid webhook URL")
		}
	case store.ChannelTypePush:
		// Push tokens are opaque strings, just ensure it's not empty
	default:
		return errors.New("invalid channel type")
	}
	return nil
}

func (client *APIClient) handleCreateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := validateTarget(req.Type, req.Target); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.SiteID != nil {
		if _, err := client.store.GetSiteByID(r.Context(), userID, *req.SiteID); err != nil {
			if errors.Is(err, store.ErrSiteNotFound) {
				writeError(w, http.StatusBadRequest, "Invalid site_id or site not owned by user")
				return
			}
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
	}

	enabled := true
	if req.IsEnabled != nil {
		enabled = *req.IsEnabled
	}

	ch := &store.NotificationChannel{
		UserID:    userID,
		SiteID:    req.SiteID,
		Type:      req.Type,
		Target:    strings.TrimSpace(req.Target),
		IsEnabled: enabled,
	}

	if err := client.store.CreateNotificationChannel(r.Context(), ch); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create channel")
		return
	}

	writeJSON(w, http.StatusCreated, ChannelResponse{Channel: ch})
}

func (client *APIClient) handleListNotificationChannels(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var siteIDFilter *uuid.UUID
	siteIDStr := r.URL.Query().Get("site_id")
	if siteIDStr != "" {
		id, err := uuid.Parse(siteIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid site_id query parameter")
			return
		}
		siteIDFilter = &id
	}

	channels, err := client.store.ListNotificationChannels(r.Context(), userID, siteIDFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, ChannelsListResponse{Channels: channels})
}

func (client *APIClient) handleUpdateNotificationChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	channelID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate new target if provided
	if req.Target != nil {
		// We need to fetch the existing channel to know its type
		existing, err := client.store.GetNotificationChannelByID(r.Context(), userID, channelID)
		if err != nil {
			if errors.Is(err, store.ErrChannelNotFound) {
				writeError(w, http.StatusNotFound, "Channel not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "Internal server error")
			return
		}

		if err := validateTarget(existing.Type, *req.Target); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		trimmed := strings.TrimSpace(*req.Target)
		req.Target = &trimmed
	}

	updated, err := client.store.UpdateNotificationChannel(r.Context(), userID, channelID, req.IsEnabled, req.Target)
	if err != nil {
		if errors.Is(err, store.ErrChannelNotFound) {
			writeError(w, http.StatusNotFound, "Channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update channel")
		return
	}

	writeJSON(w, http.StatusOK, ChannelResponse{Channel: updated})
}

func (client *APIClient) handleDeleteNotificationChannel(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	channelID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid channel ID")
		return
	}

	if err := client.store.DeleteNotificationChannel(r.Context(), userID, channelID); err != nil {
		if errors.Is(err, store.ErrChannelNotFound) {
			writeError(w, http.StatusNotFound, "Channel not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete channel")
		return
	}

	writeJSON(w, http.StatusOK, MessageResponse{Message: "Notification channel deleted successfully"})
}

func (client *APIClient) handleListNotificationEvents(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok || userID == uuid.Nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	events, err := client.store.ListNotificationEvents(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch notification events")
		return
	}

	writeJSON(w, http.StatusOK, EventsListResponse{
		Events: events,
		Limit:  limit,
		Offset: offset,
	})
}
