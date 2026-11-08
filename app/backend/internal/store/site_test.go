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

func runSiteStoreTests(t *testing.T, s store.Store) {
	ctx := context.Background()

	// 1. Setup users
	user1 := &store.User{
		ID:        uuid.New(),
		Email:     "user1-" + uuid.New().String()[:8] + "@example.com",
		CreatedAt: time.Now().UTC(),
	}
	user2 := &store.User{
		ID:        uuid.New(),
		Email:     "user2-" + uuid.New().String()[:8] + "@example.com",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateUser(ctx, user1); err != nil {
		t.Fatalf("failed to create user 1: %v", err)
	}
	if err := s.CreateUser(ctx, user2); err != nil {
		t.Fatalf("failed to create user 2: %v", err)
	}

	label := "Primary API"
	site1 := &store.Site{
		UserID:            user1.ID,
		URL:               "https://example.com/api",
		NormalizedURL:     "example.com/api",
		Label:             &label,
		CheckIntervalSec:  30,
		TimeoutSec:        5,
		FailureThreshold:  3,
		RetryCount:        2,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
	}

	// 2. Create Site with tags
	created, err := s.CreateSite(ctx, site1, []string{"prod", "api", "core"})
	if err != nil {
		t.Fatalf("failed to create site: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected non-nil site ID")
	}
	if created.CurrentStatus != store.SiteStatusPending {
		t.Errorf("expected initial status pending, got %s", created.CurrentStatus)
	}
	if len(created.Tags) != 3 {
		t.Errorf("expected 3 tags, got %v", created.Tags)
	}

	// 3. Duplicate active site for same user fails
	dupSite := &store.Site{
		UserID:        user1.ID,
		URL:           "https://example.com/api?extra=1",
		NormalizedURL: "example.com/api",
	}
	_, err = s.CreateSite(ctx, dupSite, nil)
	if err != store.ErrDuplicateSite {
		t.Errorf("expected ErrDuplicateSite, got: %v", err)
	}

	// 4. Same normalized URL for user 2 succeeds
	user2Site := &store.Site{
		UserID:        user2.ID,
		URL:           "https://example.com/api",
		NormalizedURL: "example.com/api",
	}
	createdUser2, err := s.CreateSite(ctx, user2Site, []string{"user2-tag"})
	if err != nil {
		t.Fatalf("expected user 2 to be able to monitor same URL: %v", err)
	}

	// 5. GetSiteByID
	fetched, err := s.GetSiteByID(ctx, user1.ID, created.ID)
	if err != nil {
		t.Fatalf("failed to get site by ID: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("expected ID %v, got %v", created.ID, fetched.ID)
	}
	if fetched.Label == nil || *fetched.Label != label {
		t.Errorf("expected label %s, got %v", label, fetched.Label)
	}

	// Cannot access other user's site
	_, err = s.GetSiteByID(ctx, user1.ID, createdUser2.ID)
	if err != store.ErrSiteNotFound {
		t.Errorf("expected ErrSiteNotFound for other user's site, got: %v", err)
	}

	// 6. ListSites
	user1Sites, err := s.ListSites(ctx, user1.ID, "")
	if err != nil {
		t.Fatalf("failed to list sites: %v", err)
	}
	if len(user1Sites) != 1 {
		t.Fatalf("expected 1 site for user 1, got %d", len(user1Sites))
	}

	// Tag filtering
	matched, err := s.ListSites(ctx, user1.ID, "api")
	if err != nil || len(matched) != 1 {
		t.Errorf("expected 1 matching site for tag 'api', got %d (err: %v)", len(matched), err)
	}
	unmatched, err := s.ListSites(ctx, user1.ID, "nonexistent-tag")
	if err != nil || len(unmatched) != 0 {
		t.Errorf("expected 0 matching sites for non-existent tag, got %d", len(unmatched))
	}

	// 7. UpdateSite
	newLabel := "Updated API"
	newInterval := 120
	updated, err := s.UpdateSite(ctx, user1.ID, created.ID, store.UpdateSiteInput{
		Label:            &newLabel,
		CheckIntervalSec: &newInterval,
		Tags:             &[]string{"staging", "api"},
	})
	if err != nil {
		t.Fatalf("failed to update site: %v", err)
	}
	if updated.Label == nil || *updated.Label != newLabel {
		t.Errorf("expected label %s, got %v", newLabel, updated.Label)
	}
	if updated.CheckIntervalSec != 120 {
		t.Errorf("expected interval 120, got %d", updated.CheckIntervalSec)
	}
	if len(updated.Tags) != 2 {
		t.Errorf("expected 2 updated tags, got %v", updated.Tags)
	}

	// Cannot update other user's site
	_, err = s.UpdateSite(ctx, user2.ID, created.ID, store.UpdateSiteInput{Label: &newLabel})
	if err != store.ErrSiteNotFound {
		t.Errorf("expected ErrSiteNotFound updating other user site, got: %v", err)
	}

	// 8. DeleteSite (soft delete)
	if err := s.DeleteSite(ctx, user1.ID, created.ID); err != nil {
		t.Fatalf("failed to delete site: %v", err)
	}

	// Cannot get deleted site
	_, err = s.GetSiteByID(ctx, user1.ID, created.ID)
	if err != store.ErrSiteNotFound {
		t.Errorf("expected ErrSiteNotFound for deleted site, got: %v", err)
	}

	// Deleted site excluded from list
	afterDel, err := s.ListSites(ctx, user1.ID, "")
	if err != nil || len(afterDel) != 0 {
		t.Errorf("expected 0 active sites after delete, got %d", len(afterDel))
	}

	// 9. Re-adding the same normalized URL after deletion succeeds
	reAddSite := &store.Site{
		UserID:            user1.ID,
		URL:               site1.URL,
		NormalizedURL:     site1.NormalizedURL,
		CheckIntervalSec:  30,
		TimeoutSec:        5,
		FailureThreshold:  3,
		RetryCount:        2,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
	}
	reAdded, err := s.CreateSite(ctx, reAddSite, []string{"re-added"})
	if err != nil {
		t.Fatalf("expected to be able to re-create deleted URL: %v", err)
	}
	if reAdded.ID == created.ID {
		t.Error("re-added site should have a new ID")
	}
}

func runCheckStoreTests(t *testing.T, s store.Store) {
	ctx := context.Background()

	// 1. Create a user and a site
	user := &store.User{
		ID:        uuid.New(),
		Email:     "checkuser-" + uuid.New().String()[:8] + "@example.com",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	site := &store.Site{
		UserID:        user.ID,
		URL:           "https://testcheck.com",
		NormalizedURL: "testcheck.com",
	}
	createdSite, err := s.CreateSite(ctx, site, nil)
	if err != nil {
		t.Fatalf("failed to create site: %v", err)
	}

	// 2. GetAllActiveSites
	activeSites, err := s.GetAllActiveSites(ctx)
	if err != nil {
		t.Fatalf("failed to get all active sites: %v", err)
	}
	var foundActive bool
	for _, as := range activeSites {
		if as.ID == createdSite.ID {
			foundActive = true
			break
		}
	}
	if !foundActive {
		t.Errorf("expected site %v in active sites", createdSite.ID)
	}

	// 3. RecordCheckResult
	statusCode := 200
	responseTime := 125
	checkResult := &store.CheckResult{
		SiteID:         createdSite.ID,
		CheckedAt:      time.Now().UTC(),
		StatusCode:     &statusCode,
		ResponseTimeMs: &responseTime,
		IsUp:           true,
	}
	if err := s.RecordCheckResult(ctx, checkResult); err != nil {
		t.Fatalf("failed to record check result: %v", err)
	}
	if checkResult.ID == 0 {
		t.Error("expected checkResult ID to be populated")
	}

	// 4. GetCheckResults
	results, err := s.GetCheckResults(ctx, createdSite.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get check results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 check result, got %d", len(results))
	}
	if *results[0].StatusCode != 200 || !results[0].IsUp {
		t.Errorf("unexpected check result content: %+v", results[0])
	}

	// 5. UpdateSiteState and GetSiteState
	now := time.Now().UTC()
	state := &store.SiteState{
		SiteID:              createdSite.ID,
		CurrentStatus:       store.SiteStatusUp,
		ConsecutiveFailures: 0,
		LastCheckedAt:       &now,
		LastStatusChangeAt:  &now,
	}
	if err := s.UpdateSiteState(ctx, state); err != nil {
		t.Fatalf("failed to update site state: %v", err)
	}

	fetchedState, err := s.GetSiteState(ctx, createdSite.ID)
	if err != nil {
		t.Fatalf("failed to get site state: %v", err)
	}
	if fetchedState.CurrentStatus != store.SiteStatusUp {
		t.Errorf("expected status 'up', got %s", fetchedState.CurrentStatus)
	}
	if fetchedState.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 failures, got %d", fetchedState.ConsecutiveFailures)
	}
}

func TestMemoryStoreSites(t *testing.T) {
	memStore := store.NewMemoryStore()
	runSiteStoreTests(t, memStore)
	runCheckStoreTests(t, memStore)
}

func TestPostgresStoreSites(t *testing.T) {
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
	runSiteStoreTests(t, pgStore)
	runCheckStoreTests(t, pgStore)
}
