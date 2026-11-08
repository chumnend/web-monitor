package store_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"web-monitor/internal/store"
)

func runNotificationStoreTests(t *testing.T, s store.Store) {
	ctx := context.Background()

	// 1. Setup user and site
	user := &store.User{
		ID:        uuid.New(),
		Email:     "notify-" + uuid.New().String()[:8] + "@example.com",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	site1 := &store.Site{
		UserID:        user.ID,
		URL:           "https://test-site-1.com",
		NormalizedURL: "test-site-1.com",
	}
	createdSite1, err := s.CreateSite(ctx, site1, nil)
	if err != nil {
		t.Fatalf("failed to create site 1: %v", err)
	}

	site2 := &store.Site{
		UserID:        user.ID,
		URL:           "https://test-site-2.com",
		NormalizedURL: "test-site-2.com",
	}
	createdSite2, err := s.CreateSite(ctx, site2, nil)
	if err != nil {
		t.Fatalf("failed to create site 2: %v", err)
	}

	// 2. Create Notification Channels
	globalChan := &store.NotificationChannel{
		UserID:    user.ID,
		Type:      store.ChannelTypeEmail,
		Target:    "global@example.com",
		IsEnabled: true,
	}
	if err := s.CreateNotificationChannel(ctx, globalChan); err != nil {
		t.Fatalf("failed to create global channel: %v", err)
	}
	if globalChan.ID == uuid.Nil {
		t.Fatal("expected ID to be populated on global channel")
	}

	specificChan := &store.NotificationChannel{
		UserID:    user.ID,
		SiteID:    &createdSite1.ID,
		Type:      store.ChannelTypeWebhook,
		Target:    "https://hook.example.com",
		IsEnabled: true,
	}
	if err := s.CreateNotificationChannel(ctx, specificChan); err != nil {
		t.Fatalf("failed to create specific channel: %v", err)
	}

	// 3. GetNotificationChannelByID
	fetchedGlobal, err := s.GetNotificationChannelByID(ctx, user.ID, globalChan.ID)
	if err != nil {
		t.Fatalf("failed to fetch global channel: %v", err)
	}
	if fetchedGlobal.Target != "global@example.com" {
		t.Errorf("expected target global@example.com, got %s", fetchedGlobal.Target)
	}

	// Cannot fetch with wrong user
	_, err = s.GetNotificationChannelByID(ctx, uuid.New(), globalChan.ID)
	if err != store.ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound for wrong user, got %v", err)
	}

	// 4. ListNotificationChannels
	allChannels, err := s.ListNotificationChannels(ctx, user.ID, nil)
	if err != nil || len(allChannels) != 2 {
		t.Fatalf("expected 2 channels total, got %d (err: %v)", len(allChannels), err)
	}

	site1Channels, err := s.ListNotificationChannels(ctx, user.ID, &createdSite1.ID)
	if err != nil || len(site1Channels) != 1 {
		t.Fatalf("expected 1 specific channel for site 1, got %d", len(site1Channels))
	}
	if site1Channels[0].ID != specificChan.ID {
		t.Errorf("expected site1 channel to be the specific one")
	}

	// 5. UpdateNotificationChannel
	disabled := false
	newTarget := "updated@example.com"
	updated, err := s.UpdateNotificationChannel(ctx, user.ID, globalChan.ID, &disabled, &newTarget)
	if err != nil {
		t.Fatalf("failed to update global channel: %v", err)
	}
	if updated.IsEnabled != false || updated.Target != newTarget {
		t.Errorf("failed to update channel properly: %+v", updated)
	}

	// Re-enable for further tests
	enabled := true
	_, _ = s.UpdateNotificationChannel(ctx, user.ID, globalChan.ID, &enabled, nil)

	// 6. GetChannelsForSiteTransition
	// Site 1 should get both global and specific
	transitionChans1, err := s.GetChannelsForSiteTransition(ctx, &createdSite1.Site)
	if err != nil || len(transitionChans1) != 2 {
		for i, c := range transitionChans1 {
			t.Logf("Ch %d: id=%v siteID=%v isEnabled=%v", i, c.ID, c.SiteID, c.IsEnabled)
		}
		t.Fatalf("expected 2 channels for site 1 transition, got %d", len(transitionChans1))
	}
	// Site 2 should get ONLY global
	transitionChans2, err := s.GetChannelsForSiteTransition(ctx, &createdSite2.Site)
	if err != nil || len(transitionChans2) != 1 {
		t.Fatalf("expected 1 channel for site 2 transition, got %d", len(transitionChans2))
	}
	if transitionChans2[0].ID != globalChan.ID {
		t.Errorf("expected global channel for site 2, got %v", transitionChans2[0].ID)
	}

	// 7. Record and Get Notification Events
	event1 := &store.NotificationEvent{
		SiteID:     createdSite1.ID,
		ChannelID:  globalChan.ID,
		Transition: store.TransitionUpToDown,
	}
	if err := s.RecordNotificationEvent(ctx, event1); err != nil {
		t.Fatalf("failed to record event: %v", err)
	}

	// GetLastNotificationEvent
	last, err := s.GetLastNotificationEvent(ctx, createdSite1.ID, globalChan.ID)
	if err != nil {
		t.Fatalf("failed to get last event: %v", err)
	}
	if last == nil || last.ID != event1.ID {
		t.Errorf("failed to retrieve last event properly")
	}

	// Empty last event
	empty, err := s.GetLastNotificationEvent(ctx, createdSite1.ID, uuid.New())
	if err != nil || empty != nil {
		t.Errorf("expected nil for non-existent event, got %v (err: %v)", empty, err)
	}

	// ListNotificationEvents
	events, err := s.ListNotificationEvents(ctx, user.ID, 10, 0)
	if err != nil || len(events) != 1 {
		t.Fatalf("expected 1 event for user, got %d", len(events))
	}

	// 8. DeleteNotificationChannel
	if err := s.DeleteNotificationChannel(ctx, user.ID, specificChan.ID); err != nil {
		t.Fatalf("failed to delete channel: %v", err)
	}

	_, err = s.GetNotificationChannelByID(ctx, user.ID, specificChan.ID)
	if err != store.ErrChannelNotFound {
		t.Errorf("expected ErrChannelNotFound after delete, got %v", err)
	}
}

func TestMemoryStoreNotifications(t *testing.T) {
	memStore := store.NewMemoryStore()
	runNotificationStoreTests(t, memStore)
}

func TestPostgresStoreNotifications(t *testing.T) {
	dbURL := os.Getenv("TEST_PG_URL")
	if dbURL == "" {
		dbURL = os.Getenv("PG_URL")
	}
	if dbURL == "" {
		t.Skip("skipping PostgresStore test: neither TEST_PG_URL nor PG_URL is set")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		t.Skipf("skipping PostgresStore test: ping failed: %v", err)
	}

	pgStore := store.NewPostgresStore(db)
	runNotificationStoreTests(t, pgStore)
}
