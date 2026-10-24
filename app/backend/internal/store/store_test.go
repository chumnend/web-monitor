package store_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"web-monitor/internal/store"
)

func TestUserModel(t *testing.T) {
	id := uuid.New()
	now := time.Now().Truncate(time.Second).UTC()
	verifiedAt := now.Add(-time.Hour)

	user := store.User{
		ID:              id,
		Email:           "user@example.com",
		PasswordHash:    "hashedpassword123",
		Timezone:        "America/New_York",
		EmailVerifiedAt: &verifiedAt,
		CreatedAt:       now,
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("failed to marshal User: %v", err)
	}

	var decoded store.User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal User: %v", err)
	}

	if decoded.ID != id {
		t.Errorf("expected ID %v, got %v", id, decoded.ID)
	}
	if decoded.Email != user.Email {
		t.Errorf("expected Email %v, got %v", user.Email, decoded.Email)
	}
	if decoded.Timezone != "America/New_York" {
		t.Errorf("expected Timezone %v, got %v", "America/New_York", decoded.Timezone)
	}
	if decoded.EmailVerifiedAt == nil || !decoded.EmailVerifiedAt.Equal(verifiedAt) {
		t.Errorf("expected EmailVerifiedAt %v, got %v", verifiedAt, decoded.EmailVerifiedAt)
	}
	if !decoded.CreatedAt.Equal(now) {
		t.Errorf("expected CreatedAt %v, got %v", now, decoded.CreatedAt)
	}
}

func TestUserModelNullable(t *testing.T) {
	user := store.User{
		ID:           uuid.New(),
		Email:        "unverified@example.com",
		PasswordHash: "hashed",
		Timezone:     "UTC",
		CreatedAt:    time.Now().Truncate(time.Second).UTC(),
	}

	data, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("failed to marshal User: %v", err)
	}

	var decoded store.User
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal User: %v", err)
	}

	if decoded.EmailVerifiedAt != nil {
		t.Errorf("expected EmailVerifiedAt to be nil, got %v", decoded.EmailVerifiedAt)
	}
}

func TestSiteModel(t *testing.T) {
	siteID := uuid.New()
	userID := uuid.New()
	now := time.Now().Truncate(time.Second).UTC()
	label := "My Production API"
	deletedAt := now.Add(24 * time.Hour)

	site := store.Site{
		ID:                siteID,
		UserID:            userID,
		URL:               "https://example.com/api",
		NormalizedURL:     "example.com/api",
		Label:             &label,
		CheckIntervalSec:  60,
		TimeoutSec:        10,
		FailureThreshold:  2,
		RetryCount:        1,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 399,
		CreatedAt:         now,
		DeletedAt:         &deletedAt,
	}

	data, err := json.Marshal(site)
	if err != nil {
		t.Fatalf("failed to marshal Site: %v", err)
	}

	var decoded store.Site
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal Site: %v", err)
	}

	if decoded.ID != siteID || decoded.UserID != userID {
		t.Errorf("UUID mismatch in decoded site: %v", decoded)
	}
	if decoded.Label == nil || *decoded.Label != label {
		t.Errorf("expected label %v, got %v", label, decoded.Label)
	}
	if decoded.CheckIntervalSec != 60 || decoded.TimeoutSec != 10 {
		t.Errorf("interval/timeout mismatch: %v", decoded)
	}
	if decoded.DeletedAt == nil || !decoded.DeletedAt.Equal(deletedAt) {
		t.Errorf("expected DeletedAt %v, got %v", deletedAt, decoded.DeletedAt)
	}
}

func TestTagsAndSiteTags(t *testing.T) {
	tagID := uuid.New()
	userID := uuid.New()
	siteID := uuid.New()

	tag := store.Tag{
		ID:     tagID,
		UserID: userID,
		Name:   "production",
	}

	data, err := json.Marshal(tag)
	if err != nil {
		t.Fatalf("failed to marshal Tag: %v", err)
	}

	var decodedTag store.Tag
	if err := json.Unmarshal(data, &decodedTag); err != nil {
		t.Fatalf("failed to unmarshal Tag: %v", err)
	}
	if decodedTag.Name != "production" {
		t.Errorf("expected tag name 'production', got %q", decodedTag.Name)
	}

	siteTag := store.SiteTag{
		SiteID: siteID,
		TagID:  tagID,
	}

	stData, err := json.Marshal(siteTag)
	if err != nil {
		t.Fatalf("failed to marshal SiteTag: %v", err)
	}

	var decodedSiteTag store.SiteTag
	if err := json.Unmarshal(stData, &decodedSiteTag); err != nil {
		t.Fatalf("failed to unmarshal SiteTag: %v", err)
	}
	if decodedSiteTag.SiteID != siteID || decodedSiteTag.TagID != tagID {
		t.Errorf("SiteTag ID mismatch: %v", decodedSiteTag)
	}
}

func TestCheckResultAndTimingBreakdown(t *testing.T) {
	siteID := uuid.New()
	checkedAt := time.Now().Truncate(time.Second).UTC()
	statusCode := 200
	respTime := 142
	errType := store.ErrorTypeTimeout
	errMsg := "context deadline exceeded"

	dns := 12
	tcp := 25
	tls := 30
	firstByte := 75
	total := 142

	tb := &store.TimingBreakdown{
		DNSLookupMs:    &dns,
		TCPConnectMs:   &tcp,
		TLSHandshakeMs: &tls,
		FirstByteMs:    &firstByte,
		TotalMs:        &total,
	}

	cr := store.CheckResult{
		ID:             1001,
		SiteID:         siteID,
		CheckedAt:      checkedAt,
		StatusCode:     &statusCode,
		ResponseTimeMs: &respTime,
		IsUp:           true,
		ErrorType:      &errType,
		ErrorMessage:   &errMsg,
	}

	if err := cr.SetTimingBreakdown(tb); err != nil {
		t.Fatalf("failed to set timing breakdown: %v", err)
	}

	data, err := json.Marshal(cr)
	if err != nil {
		t.Fatalf("failed to marshal CheckResult: %v", err)
	}

	var decoded store.CheckResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal CheckResult: %v", err)
	}

	if decoded.ID != 1001 || decoded.SiteID != siteID {
		t.Errorf("ID or SiteID mismatch: %v", decoded)
	}
	if decoded.StatusCode == nil || *decoded.StatusCode != 200 {
		t.Errorf("expected status code 200, got %v", decoded.StatusCode)
	}
	if decoded.IsUp != true {
		t.Errorf("expected IsUp true, got %v", decoded.IsUp)
	}

	parsedTB, err := decoded.ParseTimingBreakdown()
	if err != nil {
		t.Fatalf("failed to parse timing breakdown: %v", err)
	}
	if parsedTB == nil || *parsedTB.TotalMs != total || *parsedTB.DNSLookupMs != dns {
		t.Errorf("timing breakdown mismatch: %+v", parsedTB)
	}

	// Test SetTimingBreakdown nil
	if err := cr.SetTimingBreakdown(nil); err != nil {
		t.Fatalf("failed to set nil timing breakdown: %v", err)
	}
	if cr.TimingBreakdown != nil {
		t.Errorf("expected TimingBreakdown to be nil, got %v", cr.TimingBreakdown)
	}
	nilTB, err := cr.ParseTimingBreakdown()
	if err != nil || nilTB != nil {
		t.Errorf("expected nil timing breakdown from empty field, got %v, err %v", nilTB, err)
	}
}

func TestTimingBreakdownScanAndValue(t *testing.T) {
	dns := 15
	tb := store.TimingBreakdown{
		DNSLookupMs: &dns,
	}

	val, err := tb.Value()
	if err != nil {
		t.Fatalf("failed to get driver Value: %v", err)
	}

	// Scan from []byte
	var scanned store.TimingBreakdown
	if err := scanned.Scan(val); err != nil {
		t.Fatalf("failed to scan from []byte: %v", err)
	}
	if scanned.DNSLookupMs == nil || *scanned.DNSLookupMs != 15 {
		t.Errorf("expected DNSLookupMs 15, got %v", scanned.DNSLookupMs)
	}

	// Scan from string
	var scannedStr store.TimingBreakdown
	if err := scannedStr.Scan(string(val.([]byte))); err != nil {
		t.Fatalf("failed to scan from string: %v", err)
	}
	if scannedStr.DNSLookupMs == nil || *scannedStr.DNSLookupMs != 15 {
		t.Errorf("expected DNSLookupMs 15 from string scan, got %v", scannedStr.DNSLookupMs)
	}

	// Scan from nil
	var scannedNil store.TimingBreakdown
	if err := scannedNil.Scan(nil); err != nil {
		t.Fatalf("failed to scan from nil: %v", err)
	}

	// Scan from unsupported type
	if err := scannedNil.Scan(12345); err == nil {
		t.Errorf("expected error scanning int into TimingBreakdown, got nil")
	}
}

func TestSiteStateModel(t *testing.T) {
	siteID := uuid.New()
	now := time.Now().Truncate(time.Second).UTC()
	uptime24h := 99.95
	uptime30d := 99.80

	state := store.SiteState{
		SiteID:              siteID,
		CurrentStatus:       store.SiteStatusUp,
		ConsecutiveFailures: 0,
		LastCheckedAt:       &now,
		LastStatusChangeAt:  &now,
		UptimePct24h:        &uptime24h,
		UptimePct30d:        &uptime30d,
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("failed to marshal SiteState: %v", err)
	}

	var decoded store.SiteState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal SiteState: %v", err)
	}

	if decoded.SiteID != siteID {
		t.Errorf("expected SiteID %v, got %v", siteID, decoded.SiteID)
	}
	if decoded.CurrentStatus != store.SiteStatusUp {
		t.Errorf("expected CurrentStatus %v, got %v", store.SiteStatusUp, decoded.CurrentStatus)
	}
	if decoded.UptimePct24h == nil || *decoded.UptimePct24h != uptime24h {
		t.Errorf("expected UptimePct24h %v, got %v", uptime24h, decoded.UptimePct24h)
	}
}

func TestNotificationChannelModel(t *testing.T) {
	channelID := uuid.New()
	userID := uuid.New()
	siteID := uuid.New()
	now := time.Now().Truncate(time.Second).UTC()

	// Channel scoped to a specific site
	channel := store.NotificationChannel{
		ID:        channelID,
		UserID:    userID,
		SiteID:    &siteID,
		Type:      store.ChannelTypeEmail,
		Target:    "alerts@example.com",
		IsEnabled: true,
		CreatedAt: now,
	}

	data, err := json.Marshal(channel)
	if err != nil {
		t.Fatalf("failed to marshal NotificationChannel: %v", err)
	}

	var decoded store.NotificationChannel
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal NotificationChannel: %v", err)
	}

	if decoded.ID != channelID || decoded.UserID != userID || decoded.SiteID == nil || *decoded.SiteID != siteID {
		t.Errorf("ID mismatch: %+v", decoded)
	}
	if decoded.Type != store.ChannelTypeEmail {
		t.Errorf("expected ChannelTypeEmail, got %v", decoded.Type)
	}
	if decoded.Target != "alerts@example.com" {
		t.Errorf("expected alerts@example.com, got %v", decoded.Target)
	}

	// Global channel with site_id = nil
	globalChannel := store.NotificationChannel{
		ID:        uuid.New(),
		UserID:    userID,
		SiteID:    nil,
		Type:      store.ChannelTypeWebhook,
		Target:    "https://hooks.example.com/services/123",
		IsEnabled: true,
		CreatedAt: now,
	}

	gData, err := json.Marshal(globalChannel)
	if err != nil {
		t.Fatalf("failed to marshal global NotificationChannel: %v", err)
	}

	var decodedGlobal store.NotificationChannel
	if err := json.Unmarshal(gData, &decodedGlobal); err != nil {
		t.Fatalf("failed to unmarshal global NotificationChannel: %v", err)
	}
	if decodedGlobal.SiteID != nil {
		t.Errorf("expected SiteID to be nil for global channel, got %v", decodedGlobal.SiteID)
	}
}

func TestNotificationEventModel(t *testing.T) {
	eventID := uuid.New()
	siteID := uuid.New()
	channelID := uuid.New()
	now := time.Now().Truncate(time.Second).UTC()
	status := store.DeliveryStatusSent

	event := store.NotificationEvent{
		ID:             eventID,
		SiteID:         siteID,
		ChannelID:      channelID,
		Transition:     store.TransitionUpToDown,
		SentAt:         now,
		DeliveryStatus: &status,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal NotificationEvent: %v", err)
	}

	var decoded store.NotificationEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal NotificationEvent: %v", err)
	}

	if decoded.ID != eventID || decoded.SiteID != siteID || decoded.ChannelID != channelID {
		t.Errorf("ID mismatch: %+v", decoded)
	}
	if decoded.Transition != store.TransitionUpToDown {
		t.Errorf("expected TransitionUpToDown, got %v", decoded.Transition)
	}
	if decoded.DeliveryStatus == nil || *decoded.DeliveryStatus != store.DeliveryStatusSent {
		t.Errorf("expected delivery status 'sent', got %v", decoded.DeliveryStatus)
	}
}
