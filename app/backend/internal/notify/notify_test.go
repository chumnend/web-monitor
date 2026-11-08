package notify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/notify"
	"web-monitor/internal/store"
)

func TestWebhookSender(t *testing.T) {
	var payload []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var err error
		payload = make([]byte, r.ContentLength)
		_, err = r.Body.Read(payload)
		if err != nil && err.Error() != "EOF" {
			t.Fatalf("failed to read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	sender := notify.NewWebhookSender(ts.Client())
	msg := notify.Message{
		SiteName:   "Test Site",
		SiteURL:    "https://test.com",
		Transition: store.TransitionUpToDown,
		Timestamp:  time.Now().UTC(),
	}

	if err := sender.Send(context.Background(), ts.URL, msg); err != nil {
		t.Fatalf("webhook send failed: %v", err)
	}

	payloadStr := string(payload)
	if !contains(payloadStr, `"site_name":"Test Site"`) {
		t.Errorf("missing site_name in payload: %s", payloadStr)
	}
	if !contains(payloadStr, `"transition":"up_to_down"`) {
		t.Errorf("missing transition in payload: %s", payloadStr)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && s != "" && substr != "" && s != substr && string(s) != "" && string(substr) != "" && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

func TestDispatcherAndDebounce(t *testing.T) {
	memStore := store.NewMemoryStore()
	ctx := context.Background()

	// 1. Setup User and Site
	userID := uuid.New()
	user := &store.User{
		ID:        userID,
		Email:     "notifytest@example.com",
		CreatedAt: time.Now().UTC(),
	}
	_ = memStore.CreateUser(ctx, user)

	site := &store.Site{
		ID:            uuid.New(),
		UserID:        userID,
		URL:           "https://flapping-site.com",
		NormalizedURL: "flapping-site.com",
	}
	_, _ = memStore.CreateSite(ctx, site, nil)

	// 2. Create Notification Channel
	ch := &store.NotificationChannel{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      store.ChannelTypeWebhook,
		Target:    "https://dummy.com/hook",
		IsEnabled: true,
	}
	_ = memStore.CreateNotificationChannel(ctx, ch)

	// 3. Setup Dispatcher
	dispatcher := notify.NewDispatcher(memStore, 5*time.Minute)
	mockSender := &notify.MockSender{}
	dispatcher.RegisterSender(store.ChannelTypeWebhook, mockSender)

	// 4. First Transition -> Should be dispatched
	dispatcher.HandleTransition(site, store.TransitionUpToDown, store.SiteStatusUp)

	// Verify it was sent
	events1, _ := memStore.ListNotificationEvents(ctx, userID, 10, 0)
	if len(events1) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events1))
	}
	if *events1[0].DeliveryStatus != store.DeliveryStatusSent {
		t.Errorf("expected status 'sent', got %v", *events1[0].DeliveryStatus)
	}
	if len(mockSender.Sent) != 1 {
		t.Errorf("expected 1 msg sent to mock, got %d", len(mockSender.Sent))
	}

	// 5. Immediate Second Transition (Flapping) -> Should be DEBOUNCED
	dispatcher.HandleTransition(site, store.TransitionDownToUp, store.SiteStatusDown)

	events2, _ := memStore.ListNotificationEvents(ctx, userID, 10, 0)
	if len(events2) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events2))
	}

	// First event in array (newest) should be suppressed
	newest := events2[0]
	if *newest.DeliveryStatus != store.DeliveryStatusSuppressedDebounce {
		t.Errorf("expected status 'suppressed_debounce', got %v", *newest.DeliveryStatus)
	}
	if newest.Transition != store.TransitionDownToUp {
		t.Errorf("expected down_to_up, got %s", newest.Transition)
	}
	if len(mockSender.Sent) != 1 {
		t.Errorf("mock sender should not have received a second message, got %d", len(mockSender.Sent))
	}
}
