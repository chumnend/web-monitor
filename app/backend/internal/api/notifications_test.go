package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"web-monitor/internal/api"
	"web-monitor/internal/store"
)

func TestNotificationChannelsCRUD(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token := registerAndLogin(t, server.URL, "notify@example.com", "Password123!")

	// Create Site for specific channel test
	siteResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token, api.CreateSiteRequest{
		URL: "https://mysite.com",
	})
	var siteData api.SiteResponse
	_ = json.NewDecoder(siteResp.Body).Decode(&siteData)
	siteResp.Body.Close()
	siteID := siteData.Site.ID

	// 1. Create Webhook Channel
	whReq := api.CreateChannelRequest{
		Type:   store.ChannelTypeWebhook,
		Target: "https://webhook.site/123",
	}
	whResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/notifications", token, whReq)
	defer whResp.Body.Close()
	if whResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for webhook, got %d", whResp.StatusCode)
	}

	var whData api.ChannelResponse
	_ = json.NewDecoder(whResp.Body).Decode(&whData)
	if whData.Channel.Type != store.ChannelTypeWebhook || whData.Channel.Target != "https://webhook.site/123" {
		t.Errorf("unexpected webhook channel data: %+v", whData.Channel)
	}
	whID := whData.Channel.ID

	// 2. Create Email Channel (Site-specific)
	emailReq := api.CreateChannelRequest{
		SiteID: &siteID,
		Type:   store.ChannelTypeEmail,
		Target: "alerts@example.com",
	}
	emailResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/notifications", token, emailReq)
	defer emailResp.Body.Close()
	if emailResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for email, got %d", emailResp.StatusCode)
	}

	// 3. Validation Errors
	badEmailReq := api.CreateChannelRequest{
		Type:   store.ChannelTypeEmail,
		Target: "not-an-email",
	}
	badEmailResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/notifications", token, badEmailReq)
	badEmailResp.Body.Close()
	if badEmailResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad email, got %d", badEmailResp.StatusCode)
	}

	badWhReq := api.CreateChannelRequest{
		Type:   store.ChannelTypeWebhook,
		Target: "ftp://invalid-url.com",
	}
	badWhResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/notifications", token, badWhReq)
	badWhResp.Body.Close()
	if badWhResp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad webhook, got %d", badWhResp.StatusCode)
	}

	// 4. List Channels
	listResp := doAuthRequest(t, http.MethodGet, server.URL+"/api/notifications", token, nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for list, got %d", listResp.StatusCode)
	}
	var listData api.ChannelsListResponse
	_ = json.NewDecoder(listResp.Body).Decode(&listData)
	if len(listData.Channels) != 2 {
		t.Errorf("expected 2 channels, got %d", len(listData.Channels))
	}

	// Filter by siteID
	listSiteResp := doAuthRequest(t, http.MethodGet, server.URL+"/api/notifications?site_id="+siteID.String(), token, nil)
	defer listSiteResp.Body.Close()
	var listSiteData api.ChannelsListResponse
	_ = json.NewDecoder(listSiteResp.Body).Decode(&listSiteData)
	if len(listSiteData.Channels) != 1 || listSiteData.Channels[0].Type != store.ChannelTypeEmail {
		t.Errorf("expected 1 email channel filtered by site, got %+v", listSiteData.Channels)
	}

	// 5. Update Channel
	disabled := false
	newTarget := "https://webhook.site/456"
	updateReq := api.UpdateChannelRequest{
		IsEnabled: &disabled,
		Target:    &newTarget,
	}
	updateResp := doAuthRequest(t, http.MethodPatch, server.URL+"/api/notifications/"+whID.String(), token, updateReq)
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for update, got %d", updateResp.StatusCode)
	}
	var updateData api.ChannelResponse
	_ = json.NewDecoder(updateResp.Body).Decode(&updateData)
	if updateData.Channel.IsEnabled != false || updateData.Channel.Target != newTarget {
		t.Errorf("expected disabled and new target, got %+v", updateData.Channel)
	}

	// 6. Delete Channel
	delResp := doAuthRequest(t, http.MethodDelete, server.URL+"/api/notifications/"+whID.String(), token, nil)
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for delete, got %d", delResp.StatusCode)
	}

	// Verify deleted
	listAfterDel := doAuthRequest(t, http.MethodGet, server.URL+"/api/notifications", token, nil)
	defer listAfterDel.Body.Close()
	var finalData api.ChannelsListResponse
	_ = json.NewDecoder(listAfterDel.Body).Decode(&finalData)
	if len(finalData.Channels) != 1 {
		t.Errorf("expected 1 channel remaining, got %d", len(finalData.Channels))
	}
}

func TestNotificationEventsEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token := registerAndLogin(t, server.URL, "events@example.com", "Password123!")

	// Instead of triggering whole dispatcher, we can mock an event in the store
	// First, need a user and site. The user ID comes from the token, but we can't extract it directly easily.
	// We'll create a site via API to get the user ID context right.
	siteResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token, api.CreateSiteRequest{
		URL: "https://events-site.com",
	})
	var siteData api.SiteResponse
	_ = json.NewDecoder(siteResp.Body).Decode(&siteData)
	siteResp.Body.Close()

	if siteData.Site == nil {
		t.Fatalf("expected non-nil siteData.Site, status: %d", siteResp.StatusCode)
	}

	// Add an event directly to store
	status := store.DeliveryStatusSent
	err := memStore.RecordNotificationEvent(context.Background(), &store.NotificationEvent{
		SiteID:         siteData.Site.ID,
		ChannelID:      uuid.New(),
		Transition:     store.TransitionUpToDown,
		DeliveryStatus: &status,
	})
	if err != nil {
		t.Fatalf("failed to mock event: %v", err)
	}

	// Request events via API
	eventsResp := doAuthRequest(t, http.MethodGet, server.URL+"/api/notifications/events", token, nil)
	defer eventsResp.Body.Close()
	if eventsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", eventsResp.StatusCode)
	}

	var eventsData api.EventsListResponse
	if err := json.NewDecoder(eventsResp.Body).Decode(&eventsData); err != nil {
		t.Fatalf("failed to decode events: %v", err)
	}
	if len(eventsData.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(eventsData.Events))
	}
	if eventsData.Events[0].Transition != store.TransitionUpToDown {
		t.Errorf("expected transition up_to_down, got %s", eventsData.Events[0].Transition)
	}
}
