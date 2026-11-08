package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"web-monitor/internal/api"
	"web-monitor/internal/store"
)

func registerAndLogin(t *testing.T, serverURL, email, password string) string {
	t.Helper()

	// Register
	regBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	regResp, err := http.Post(serverURL+"/api/auth/register", "application/json", bytes.NewReader(regBody))
	if err != nil {
		t.Fatalf("failed to register %s: %v", email, err)
	}
	regResp.Body.Close()
	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 on register %s, got %d", email, regResp.StatusCode)
	}

	// Login
	loginBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})
	loginResp, err := http.Post(serverURL+"/api/auth/login", "application/json", bytes.NewReader(loginBody))
	if err != nil {
		t.Fatalf("failed to login %s: %v", email, err)
	}
	defer loginResp.Body.Close()

	var loginData api.LoginResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&loginData); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	return loginData.Token
}

func doAuthRequest(t *testing.T, method, url, token string, body any) *http.Response {
	t.Helper()

	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = bytes.NewReader([]byte{})
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to execute request %s %s: %v", method, url, err)
	}
	return resp
}

func TestSitesCRUDFlow(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token := registerAndLogin(t, server.URL, "siteowner@example.com", "Password123!")

	// 1. Create Site
	label := "My Frontend App"
	retryCount := 2
	createReq := api.CreateSiteRequest{
		URL:               "https://frontend.example.com:443/",
		Label:             &label,
		CheckIntervalSec:  30,
		TimeoutSec:        5,
		FailureThreshold:  3,
		RetryCount:        &retryCount,
		ExpectedStatusMin: 200,
		ExpectedStatusMax: 299,
		Tags:              []string{"frontend", "prod"},
	}

	createResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token, createReq)
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created on create site, got %d", createResp.StatusCode)
	}

	var createdSite api.SiteResponse
	if err := json.NewDecoder(createResp.Body).Decode(&createdSite); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}

	site := createdSite.Site
	if site.ID == uuid.Nil {
		t.Fatal("expected non-nil site ID")
	}
	if site.URL != "https://frontend.example.com" {
		t.Errorf("expected URL https://frontend.example.com, got %s", site.URL)
	}
	if site.NormalizedURL != "frontend.example.com" {
		t.Errorf("expected NormalizedURL frontend.example.com, got %s", site.NormalizedURL)
	}
	if site.CheckIntervalSec != 30 {
		t.Errorf("expected interval 30, got %d", site.CheckIntervalSec)
	}
	if site.CurrentStatus != store.SiteStatusPending {
		t.Errorf("expected initial status pending, got %s", site.CurrentStatus)
	}
	if len(site.Tags) != 2 {
		t.Errorf("expected 2 tags, got %v", site.Tags)
	}

	// 2. List Sites
	listResp := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites", token, nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on list sites, got %d", listResp.StatusCode)
	}

	var listData api.SitesListResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listData); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listData.Sites) != 1 {
		t.Fatalf("expected 1 site in list, got %d", len(listData.Sites))
	}
	if listData.Sites[0].ID != site.ID {
		t.Errorf("expected site ID %v in list, got %v", site.ID, listData.Sites[0].ID)
	}

	// 3. Get Site by ID
	getResp := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+site.ID.String(), token, nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on get site, got %d", getResp.StatusCode)
	}

	var getData api.SiteResponse
	if err := json.NewDecoder(getResp.Body).Decode(&getData); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getData.Site.ID != site.ID {
		t.Errorf("get ID mismatch: %v vs %v", getData.Site.ID, site.ID)
	}

	// 4. Update Site (PATCH)
	newLabel := "Updated Frontend App"
	newInterval := 120
	updateReq := api.UpdateSiteRequest{
		Label:            &newLabel,
		CheckIntervalSec: &newInterval,
		Tags:             &[]string{"frontend", "v2"},
	}

	updateResp := doAuthRequest(t, http.MethodPatch, server.URL+"/api/sites/"+site.ID.String(), token, updateReq)
	defer updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on update site, got %d", updateResp.StatusCode)
	}

	var updatedData api.SiteResponse
	if err := json.NewDecoder(updateResp.Body).Decode(&updatedData); err != nil {
		t.Fatalf("failed to decode update response: %v", err)
	}
	if updatedData.Site.Label == nil || *updatedData.Site.Label != newLabel {
		t.Errorf("expected label %s, got %v", newLabel, updatedData.Site.Label)
	}
	if updatedData.Site.CheckIntervalSec != 120 {
		t.Errorf("expected interval 120, got %d", updatedData.Site.CheckIntervalSec)
	}
	if len(updatedData.Site.Tags) != 2 || updatedData.Site.Tags[1] != "v2" {
		t.Errorf("expected tags ['frontend', 'v2'], got %v", updatedData.Site.Tags)
	}

	// 5. Delete Site
	delResp := doAuthRequest(t, http.MethodDelete, server.URL+"/api/sites/"+site.ID.String(), token, nil)
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on delete site, got %d", delResp.StatusCode)
	}

	// 6. Verify deleted site returns 404
	getAfterDel := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+site.ID.String(), token, nil)
	defer getAfterDel.Body.Close()
	if getAfterDel.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got %d", getAfterDel.StatusCode)
	}

	// 7. Verify list is empty
	listAfterDel := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites", token, nil)
	defer listAfterDel.Body.Close()
	var emptyList api.SitesListResponse
	_ = json.NewDecoder(listAfterDel.Body).Decode(&emptyList)
	if len(emptyList.Sites) != 0 {
		t.Errorf("expected 0 sites after deletion, got %d", len(emptyList.Sites))
	}
}

func TestSitesAuthorization(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	user1Token := registerAndLogin(t, server.URL, "user1@example.com", "Password123!")
	user2Token := registerAndLogin(t, server.URL, "user2@example.com", "Password123!")

	// User 1 creates site
	createResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", user1Token, api.CreateSiteRequest{
		URL: "https://user1-site.com",
	})
	defer createResp.Body.Close()
	var created api.SiteResponse
	_ = json.NewDecoder(createResp.Body).Decode(&created)
	user1SiteID := created.Site.ID.String()

	// User 2 cannot access User 1's site
	getResp := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+user1SiteID, user2Token, nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for User 2 accessing User 1 site, got %d", getResp.StatusCode)
	}

	// User 2 cannot update User 1's site
	newLabel := "Hacked"
	patchResp := doAuthRequest(t, http.MethodPatch, server.URL+"/api/sites/"+user1SiteID, user2Token, api.UpdateSiteRequest{
		Label: &newLabel,
	})
	defer patchResp.Body.Close()
	if patchResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for User 2 updating User 1 site, got %d", patchResp.StatusCode)
	}

	// User 2 cannot delete User 1's site
	delResp := doAuthRequest(t, http.MethodDelete, server.URL+"/api/sites/"+user1SiteID, user2Token, nil)
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for User 2 deleting User 1 site, got %d", delResp.StatusCode)
	}

	// User 2's list does not show User 1's site
	listResp := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites", user2Token, nil)
	defer listResp.Body.Close()
	var user2List api.SitesListResponse
	_ = json.NewDecoder(listResp.Body).Decode(&user2List)
	if len(user2List.Sites) != 0 {
		t.Errorf("expected 0 sites for user 2, got %d", len(user2List.Sites))
	}
}

func TestSitesUnauthenticated(t *testing.T) {
	client := api.NewAPIClient()
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	dummyID := uuid.New().String()

	endpoints := []struct {
		method string
		url    string
	}{
		{http.MethodPost, server.URL + "/api/sites"},
		{http.MethodGet, server.URL + "/api/sites"},
		{http.MethodGet, server.URL + "/api/sites/" + dummyID},
		{http.MethodPatch, server.URL + "/api/sites/" + dummyID},
		{http.MethodPut, server.URL + "/api/sites/" + dummyID},
		{http.MethodDelete, server.URL + "/api/sites/" + dummyID},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.url, func(t *testing.T) {
			resp := doAuthRequest(t, ep.method, ep.url, "", nil)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
			}
		})
	}
}

func TestSitesValidationAndDuplicates(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token1 := registerAndLogin(t, server.URL, "dupuser1@example.com", "Password123!")
	token2 := registerAndLogin(t, server.URL, "dupuser2@example.com", "Password123!")

	// 1. Invalid URL
	resp1 := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token1, map[string]string{
		"url": "not-a-valid-url",
	})
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for bad URL, got %d", resp1.StatusCode)
	}

	// 2. Add valid site
	resp2 := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token1, map[string]string{
		"url": "https://api.domain.com/",
	})
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp2.StatusCode)
	}

	// 3. Duplicate site for same user -> 409 Conflict
	resp3 := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token1, map[string]string{
		"url": "http://api.domain.com:80",
	})
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict on duplicate normalized URL, got %d", resp3.StatusCode)
	}

	// 4. Same URL for different user -> 201 Created
	resp4 := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token2, map[string]string{
		"url": "https://api.domain.com",
	})
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 for different user adding same URL, got %d", resp4.StatusCode)
	}
}

func TestSitesTagFiltering(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token := registerAndLogin(t, server.URL, "taguser@example.com", "Password123!")

	// Create Site A with tags: "frontend", "prod"
	_ = doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token, api.CreateSiteRequest{
		URL:  "https://site-a.com",
		Tags: []string{"frontend", "prod"},
	})

	// Create Site B with tags: "backend", "prod"
	_ = doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token, api.CreateSiteRequest{
		URL:  "https://site-b.com",
		Tags: []string{"backend", "prod"},
	})

	// Filter by frontend -> only Site A
	respFE := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites?tag=frontend", token, nil)
	defer respFE.Body.Close()
	var listFE api.SitesListResponse
	_ = json.NewDecoder(respFE.Body).Decode(&listFE)
	if len(listFE.Sites) != 1 || listFE.Sites[0].URL != "https://site-a.com" {
		t.Errorf("expected only Site A for tag 'frontend', got: %+v", listFE.Sites)
	}

	// Filter by prod -> both sites
	respProd := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites?tag=prod", token, nil)
	defer respProd.Body.Close()
	var listProd api.SitesListResponse
	_ = json.NewDecoder(respProd.Body).Decode(&listProd)
	if len(listProd.Sites) != 2 {
		t.Errorf("expected 2 sites for tag 'prod', got %d", len(listProd.Sites))
	}

	// Filter by staging -> 0 sites
	respStaging := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites?tag=staging", token, nil)
	defer respStaging.Body.Close()
	var listStaging api.SitesListResponse
	_ = json.NewDecoder(respStaging.Body).Decode(&listStaging)
	if len(listStaging.Sites) != 0 {
		t.Errorf("expected 0 sites for tag 'staging', got %d", len(listStaging.Sites))
	}
}

func TestSiteHistoryEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token1 := registerAndLogin(t, server.URL, "histuser1@example.com", "Password123!")
	token2 := registerAndLogin(t, server.URL, "histuser2@example.com", "Password123!")

	// Create site
	siteResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token1, api.CreateSiteRequest{
		URL: "https://myservice.com",
	})
	defer siteResp.Body.Close()
	var siteData api.SiteResponse
	_ = json.NewDecoder(siteResp.Body).Decode(&siteData)
	siteID := siteData.Site.ID

	// Insert 3 check results
	status200 := 200
	respMs := 120
	for i := 0; i < 3; i++ {
		_ = memStore.RecordCheckResult(t.Context(), &store.CheckResult{
			SiteID:         siteID,
			StatusCode:     &status200,
			ResponseTimeMs: &respMs,
			IsUp:           true,
		})
	}

	// 1. Get history with pagination (limit=2, offset=0)
	resp1 := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+siteID.String()+"/history?limit=2&offset=0", token1, nil)
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on history, got %d", resp1.StatusCode)
	}

	var histData api.SiteHistoryResponse
	if err := json.NewDecoder(resp1.Body).Decode(&histData); err != nil {
		t.Fatalf("failed to decode history response: %v", err)
	}
	if histData.Total != 3 {
		t.Errorf("expected total 3, got %d", histData.Total)
	}
	if len(histData.Results) != 2 {
		t.Errorf("expected 2 results on page 1, got %d", len(histData.Results))
	}
	if histData.Limit != 2 || histData.Offset != 0 {
		t.Errorf("expected limit 2 and offset 0, got %d, %d", histData.Limit, histData.Offset)
	}

	// 2. User 2 cannot access history for User 1's site
	respUser2 := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+siteID.String()+"/history", token2, nil)
	defer respUser2.Body.Close()
	if respUser2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for User 2 accessing User 1 history, got %d", respUser2.StatusCode)
	}

	// 3. Unauthenticated -> 401
	respUnauth := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+siteID.String()+"/history", "", nil)
	defer respUnauth.Body.Close()
	if respUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated history, got %d", respUnauth.StatusCode)
	}
}

func TestSiteUptimeEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token1 := registerAndLogin(t, server.URL, "uptimeuser1@example.com", "Password123!")
	token2 := registerAndLogin(t, server.URL, "uptimeuser2@example.com", "Password123!")

	// Create site
	siteResp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token1, api.CreateSiteRequest{
		URL: "https://uptimetest.com",
	})
	defer siteResp.Body.Close()
	var siteData api.SiteResponse
	_ = json.NewDecoder(siteResp.Body).Decode(&siteData)
	siteID := siteData.Site.ID

	// Insert checks: 1 up (100ms), 1 down (200ms)
	s200 := 200
	r100 := 100
	_ = memStore.RecordCheckResult(t.Context(), &store.CheckResult{
		SiteID:         siteID,
		StatusCode:     &s200,
		ResponseTimeMs: &r100,
		IsUp:           true,
	})
	s500 := 500
	r200 := 200
	_ = memStore.RecordCheckResult(t.Context(), &store.CheckResult{
		SiteID:         siteID,
		StatusCode:     &s500,
		ResponseTimeMs: &r200,
		IsUp:           false,
	})

	// 1. Get uptime
	resp := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+siteID.String()+"/uptime", token1, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on uptime, got %d", resp.StatusCode)
	}

	var uptimeData store.SiteUptimeStats
	if err := json.NewDecoder(resp.Body).Decode(&uptimeData); err != nil {
		t.Fatalf("failed to decode uptime response: %v", err)
	}
	if uptimeData.TotalChecks24h != 2 {
		t.Errorf("expected 2 total checks, got %d", uptimeData.TotalChecks24h)
	}
	if uptimeData.UptimePct24h != 50.0 {
		t.Errorf("expected 50%% uptime, got %v", uptimeData.UptimePct24h)
	}

	// 2. User 2 cannot access uptime for User 1's site
	respUser2 := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+siteID.String()+"/uptime", token2, nil)
	defer respUser2.Body.Close()
	if respUser2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for User 2 accessing User 1 uptime, got %d", respUser2.StatusCode)
	}

	// 3. Unauthenticated -> 401
	respUnauth := doAuthRequest(t, http.MethodGet, server.URL+"/api/sites/"+siteID.String()+"/uptime", "", nil)
	defer respUnauth.Body.Close()
	if respUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated uptime, got %d", respUnauth.StatusCode)
	}
}

func TestDashboardSummaryEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	client := api.NewAPIClient(api.WithStore(memStore))
	defer client.Close()
	server := httptest.NewServer(client.Handler())
	defer server.Close()

	token1 := registerAndLogin(t, server.URL, "dashuser1@example.com", "Password123!")
	token2 := registerAndLogin(t, server.URL, "dashuser2@example.com", "Password123!")

	// User 1 creates 2 sites
	site1Resp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token1, api.CreateSiteRequest{
		URL: "https://dashsite1.com",
	})
	site1Resp.Body.Close()

	site2Resp := doAuthRequest(t, http.MethodPost, server.URL+"/api/sites", token1, api.CreateSiteRequest{
		URL: "https://dashsite2.com",
	})
	site2Resp.Body.Close()

	// 1. Get dashboard summary for User 1
	resp := doAuthRequest(t, http.MethodGet, server.URL+"/api/dashboard/summary", token1, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on summary, got %d", resp.StatusCode)
	}

	var summary store.DashboardSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("failed to decode dashboard summary: %v", err)
	}
	if summary.TotalSites != 2 {
		t.Errorf("expected 2 total sites, got %d", summary.TotalSites)
	}
	if summary.PendingCount != 2 {
		t.Errorf("expected 2 pending sites initially, got %d", summary.PendingCount)
	}

	// 2. User 2 has 0 sites
	respUser2 := doAuthRequest(t, http.MethodGet, server.URL+"/api/dashboard/summary", token2, nil)
	defer respUser2.Body.Close()
	var summaryUser2 store.DashboardSummary
	_ = json.NewDecoder(respUser2.Body).Decode(&summaryUser2)
	if summaryUser2.TotalSites != 0 {
		t.Errorf("expected 0 sites for user 2, got %d", summaryUser2.TotalSites)
	}

	// 3. Unauthenticated -> 401
	respUnauth := doAuthRequest(t, http.MethodGet, server.URL+"/api/dashboard/summary", "", nil)
	defer respUnauth.Body.Close()
	if respUnauth.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated summary, got %d", respUnauth.StatusCode)
	}
}

