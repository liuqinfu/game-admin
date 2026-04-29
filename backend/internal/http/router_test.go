package http

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"game-admin/backend/internal/config"
	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
)

type stubHealthProvider struct{}

func (stubHealthProvider) Live(now time.Time, requestID, traceID string) HealthResponse {
	return HealthResponse{
		Status:    "ok",
		Service:   "game-admin-backend",
		Version:   "dev",
		Timestamp: now.UTC(),
		RequestID: requestID,
		TraceID:   traceID,
	}
}

func (stubHealthProvider) Ready(now time.Time, requestID, traceID string) HealthResponse {
	return (stubHealthProvider{}).Live(now, requestID, traceID)
}

func TestNewRouterHealthz(t *testing.T) {

	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "*", resp.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header().Get("Access-Control-Allow-Methods"), http.MethodGet)

	var body HealthResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, "ok", body.Status)
	require.Equal(t, "game-admin-backend", body.Service)
	require.Equal(t, "dev", body.Version)
	require.WithinDuration(t, time.Now().UTC(), body.Timestamp, 5*time.Second)
}

func TestNewRouterHealthzRequestContextHeaders(t *testing.T) {
	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "req-test-health")
	req.Header.Set("X-Trace-ID", "trace-test-health")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "req-test-health", resp.Header().Get("X-Request-ID"))
	require.Equal(t, "trace-test-health", resp.Header().Get("X-Trace-ID"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, "req-test-health", body["requestID"])
	require.Equal(t, "trace-test-health", body["traceID"])
}

func TestNewRouterSanitizesRequestContextHeaders(t *testing.T) {
	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-ID", "   bad req\n<>id###   ")
	req.Header.Set("X-Trace-ID", strings.Repeat("t", 140)+"###")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "badreqid", resp.Header().Get("X-Request-ID"))
	require.Len(t, resp.Header().Get("X-Trace-ID"), 128)
	require.NotContains(t, resp.Header().Get("X-Trace-ID"), "#")

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, resp.Header().Get("X-Request-ID"), body["requestID"])
	require.Equal(t, resp.Header().Get("X-Trace-ID"), body["traceID"])
}

func TestNewRouterErrorResponseCarriesRequestContext(t *testing.T) {
	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}})
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("X-Request-ID", "req-test-error")
	req.Header.Set("X-Trace-ID", "trace-test-error")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusUnauthorized, resp.Code)
	require.Equal(t, "req-test-error", resp.Header().Get("X-Request-ID"))
	require.Equal(t, "trace-test-error", resp.Header().Get("X-Trace-ID"))
	assertJSONErrorBody(t, resp, "unauthorized")

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, "req-test-error", body["requestID"])
	require.Equal(t, "trace-test-error", body["traceID"])
}

func TestNewRouterGeneratesRequestContextWhenMissing(t *testing.T) {
	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.NotEmpty(t, resp.Header().Get("X-Request-ID"))
	require.NotEmpty(t, resp.Header().Get("X-Trace-ID"))

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, resp.Header().Get("X-Request-ID"), body["requestID"])
	require.Equal(t, resp.Header().Get("X-Trace-ID"), body["traceID"])
}

func TestNewRouterMetricsEndpointExposesPrometheus(t *testing.T) {
	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}})

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthResp := httptest.NewRecorder()
	router.ServeHTTP(healthResp, healthReq)
	require.Equal(t, http.StatusOK, healthResp.Code)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Header().Get("Content-Type"), "text/plain")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_http_requests_total")
	require.Contains(t, resp.Body.String(), "game_admin_runtime_ready_status")
}

func TestNewRouterInternalReportContractEndpoint(t *testing.T) {
	router, _ := newTestRouter(t)

	resp := performJSONNoAuth(t, router, http.MethodPost, "/internal/contracts/report/team-performance", map[string]any{
		"scope":    map[string]any{},
		"currency": "CNY",
	})

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		Items []map[string]any `json:"items"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.NotNil(t, body.Items)
}

func TestNewRouterInternalRiskContractEndpoint(t *testing.T) {
	router, testDB := newTestRouter(t)

	tenant := model.Tenant{Code: "RISK-TENANT", Name: "Risk Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "RISK-BRAND", Name: "Risk Brand", Status: model.BrandStatusActive, IsDefault: true}
	require.NoError(t, testDB.Create(&brand).Error)
	agent := model.Agent{TenantID: &tenant.ID, BrandID: &brand.ID, AgentNo: "RISK-AGENT", Name: "Risk Agent", Status: model.AgentStatusActive}
	require.NoError(t, testDB.Create(&agent).Error)
	account := model.AgentAccount{TenantID: &tenant.ID, BrandID: &brand.ID, AgentID: agent.ID, AccountNo: "ACC-RISK-1", Currency: "CNY", Balance: 100, AvailableBalance: 80, WithdrawableAmount: 80, FrozenBalance: 20}
	require.NoError(t, testDB.Create(&account).Error)
	riskCase := model.RiskCase{
		CaseNo:          "RC-001",
		TenantID:        &tenant.ID,
		BrandID:         &brand.ID,
		AgentID:         agent.ID,
		Currency:        "CNY",
		Amount:          20,
		Status:          model.RiskCaseStatusPending,
		FreezeRequested: true,
	}
	require.NoError(t, testDB.Create(&riskCase).Error)
	freezeLedger := model.AgentAccountLedger{
		TenantID:       &tenant.ID,
		BrandID:        &brand.ID,
		AccountID:      account.ID,
		AgentID:        agent.ID,
		ReferenceType:  "risk_case",
		ReferenceID:    riskCase.CaseNo,
		LedgerType:     model.LedgerTypeFreeze,
		Direction:      model.LedgerDirectionDebit,
		Amount:         20,
		BalanceBefore:  100,
		BalanceAfter:   100,
		FrozenBefore:   0,
		FrozenAfter:    20,
		Currency:       "CNY",
		OccurredAt:     time.Now().UTC(),
		IdempotencyKey: "risk-freeze-1",
	}
	require.NoError(t, testDB.Create(&freezeLedger).Error)

	resp := performJSONNoAuth(t, router, http.MethodPost, "/internal/contracts/risk/release-pending-cases", map[string]any{
		"scope":    map[string]any{"tenantIDs": []uint64{tenant.ID}, "brandIDs": []uint64{brand.ID}},
		"agentID":  agent.ID,
		"remark":   "paid",
		"reviewer": "finance",
	})

	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		ReleasedCount int `json:"releasedCount"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, 1, body.ReleasedCount)

	restoreResp := performJSONNoAuth(t, router, http.MethodPost, "/internal/contracts/risk/restore-pending-cases", map[string]any{
		"scope":    map[string]any{"tenantIDs": []uint64{tenant.ID}, "brandIDs": []uint64{brand.ID}},
		"caseNos":  []string{riskCase.CaseNo},
		"remark":   "rollback payout",
		"reviewer": "finance",
	})

	require.Equal(t, http.StatusOK, restoreResp.Code, restoreResp.Body.String())
	var restoreBody struct {
		RestoredCount int `json:"restoredCount"`
	}
	require.NoError(t, json.Unmarshal(restoreResp.Body.Bytes(), &restoreBody))
	require.Equal(t, 1, restoreBody.RestoredCount)

	var restored model.RiskCase
	require.NoError(t, testDB.First(&restored, riskCase.ID).Error)
	require.Equal(t, model.RiskCaseStatusPending, restored.Status)
}

func TestNewRouterHandlesCORSPreflight(t *testing.T) {

	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}})
	req := httptest.NewRequest(http.MethodOptions, "/api/orders", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	require.Equal(t, http.StatusNoContent, resp.Code)
	require.Equal(t, "*", resp.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, resp.Header().Get("Access-Control-Allow-Methods"), http.MethodGet)
	require.Contains(t, resp.Header().Get("Access-Control-Allow-Headers"), "Authorization")
}

func TestGameOpenAPIUsesSignedGameCredential(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenant := model.Tenant{Code: "OPEN-TENANT", Name: "Open Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "OPEN-BRAND", Name: "Open Brand", Status: model.BrandStatusActive, IsDefault: true}
	require.NoError(t, testDB.Create(&brand).Error)
	agent := model.Agent{TenantID: &tenant.ID, BrandID: &brand.ID, AgentNo: "OPEN-AG", Name: "Open Agent", Status: model.AgentStatusActive, Level: 1, Currency: "CNY"}
	require.NoError(t, testDB.Create(&agent).Error)
	invite := model.InviteCode{TenantID: &tenant.ID, BrandID: &brand.ID, AgentID: agent.ID, Code: "OPEN-INV", Status: model.InviteCodeStatusActive}
	require.NoError(t, testDB.Create(&invite).Error)
	game := model.Game{TenantID: &tenant.ID, BrandID: &brand.ID, GameCode: "OPEN-GAME", Name: "Open Game", Status: model.GameStatusOnline, IsAgentable: true, Currency: "CNY"}
	require.NoError(t, testDB.Create(&game).Error)
	credential := model.GameIntegrationKey{TenantID: &tenant.ID, BrandID: &brand.ID, GameID: game.ID, Name: "test", AccessKey: "ga_live_test", SecretCiphertext: "gsk_live_test", Status: model.GameIntegrationKeyStatusActive, Scopes: jsonStringArray(defaultGameOpenAPIScopes())}
	require.NoError(t, testDB.Create(&credential).Error)
	access := model.AgentGameAccess{TenantID: &tenant.ID, BrandID: &brand.ID, AgentID: agent.ID, GameID: game.ID, Status: model.AccessStatusEnabled, GrantedBy: "test", GrantedAt: time.Now().UTC(), EffectiveFrom: time.Now().UTC().Add(-time.Hour)}
	require.NoError(t, testDB.Create(&access).Error)

	unsigned := performJSONNoAuth(t, router, http.MethodPost, "/openapi/game/players/register-with-invite", map[string]any{"platformUserID": "open-user-denied", "inviteCode": invite.Code})
	require.Equal(t, http.StatusUnauthorized, unsigned.Code)

	registerPayload := map[string]any{"playerNo": "OPEN-PLAYER-1", "platformUserID": "open-user-1", "nickname": "Open User", "currency": "CNY", "inviteCode": invite.Code}
	registered := performSignedGameJSON(t, router, http.MethodPost, "/openapi/game/players/register-with-invite", credential.AccessKey, credential.SecretCiphertext, registerPayload)
	require.Equal(t, http.StatusCreated, registered.Code, registered.Body.String())
	var registerBody struct {
		Player  model.Player  `json:"player"`
		Binding model.Binding `json:"binding"`
	}
	require.NoError(t, json.Unmarshal(registered.Body.Bytes(), &registerBody))
	require.Equal(t, game.ID, *registerBody.Player.RegisterGameID)
	require.Equal(t, agent.ID, registerBody.Binding.AgentID)

	rechargePayload := map[string]any{"orderNo": "OPEN-ORDER-1", "playerID": registerBody.Player.ID, "amount": 100, "currency": "CNY", "status": "paid", "paidAt": time.Now().UTC().Format(time.RFC3339), "idempotencyKey": "open-order-1-paid"}
	recharge := performSignedGameJSON(t, router, http.MethodPost, "/openapi/game/recharge/callback", credential.AccessKey, credential.SecretCiphertext, rechargePayload)
	require.Equal(t, http.StatusCreated, recharge.Code, recharge.Body.String())
	var rechargeBody rechargeCallbackResult
	require.NoError(t, json.Unmarshal(recharge.Body.Bytes(), &rechargeBody))
	require.Equal(t, game.ID, rechargeBody.Order.GameID)
	require.Equal(t, agent.ID, *rechargeBody.Order.AgentID)
	var paidEvent model.DomainEvent
	require.NoError(t, testDB.Where("event_type = ? AND aggregate_id = ?", eventbus.EventRechargeOrderPaid, strconv.FormatUint(rechargeBody.Order.ID, 10)).First(&paidEvent).Error)
	var paidDeliveries int64
	require.NoError(t, testDB.Model(&model.DomainEventDelivery{}).Where("event_id = ?", paidEvent.ID).Count(&paidDeliveries).Error)
	require.Equal(t, int64(1), paidDeliveries)

	rechargeAgain := performSignedGameJSON(t, router, http.MethodPost, "/openapi/game/recharge/callback", credential.AccessKey, credential.SecretCiphertext, rechargePayload)
	require.Equal(t, http.StatusOK, rechargeAgain.Code, rechargeAgain.Body.String())
	var duplicateBody rechargeCallbackResult
	require.NoError(t, json.Unmarshal(rechargeAgain.Body.Bytes(), &duplicateBody))
	require.True(t, duplicateBody.Duplicate)
}

func TestGameOpenAPIEnumDictionaries(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenant := model.Tenant{Code: "ENUM-TENANT", Name: "Enum Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "ENUM-BRAND", Name: "Enum Brand", Status: model.BrandStatusActive, IsDefault: true}
	require.NoError(t, testDB.Create(&brand).Error)
	game := model.Game{TenantID: &tenant.ID, BrandID: &brand.ID, GameCode: "ENUM-GAME", Name: "Enum Game", Status: model.GameStatusOnline, IsAgentable: true, Currency: "CNY"}
	require.NoError(t, testDB.Create(&game).Error)
	credential := model.GameIntegrationKey{TenantID: &tenant.ID, BrandID: &brand.ID, GameID: game.ID, Name: "enum", AccessKey: "ga_live_enum", SecretCiphertext: "gsk_live_enum", Status: model.GameIntegrationKeyStatusActive, Scopes: jsonStringArray(defaultGameOpenAPIScopes())}
	require.NoError(t, testDB.Create(&credential).Error)

	resp := performSignedGameJSON(t, router, http.MethodGet, "/openapi/game/enum-dictionaries", credential.AccessKey, credential.SecretCiphertext, nil)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var body struct {
		Items []enumDictionaryResponse `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, 2, body.Total)
	require.Len(t, body.Items, 2)

	recharge := body.Items[0]
	if recharge.Code != enumDictionaryRechargeTypeCode {
		recharge = body.Items[1]
	}
	require.Equal(t, enumDictionaryRechargeTypeCode, recharge.Code)
	require.True(t, recharge.Strict)
	require.NotEmpty(t, recharge.Items)
}

func TestAuthLoginAndProtectedRoutes(t *testing.T) {

	router, testDB := newTestRouter(t)
	tokens := seedRBAC(t, testDB)

	unauthorized := performRequestNoAuth(t, router, http.MethodGet, "/api/orders", nil)
	require.Equal(t, http.StatusUnauthorized, unauthorized.Code)
	assertJSONErrorBody(t, unauthorized, "unauthorized")

	badLogin := performJSONNoAuth(t, router, http.MethodPost, "/api/auth/login", map[string]any{"username": "admin", "password": "wrong-password"})
	require.Equal(t, http.StatusUnauthorized, badLogin.Code)
	assertJSONErrorBody(t, badLogin, "invalid credentials")

	login := performJSONNoAuth(t, router, http.MethodPost, "/api/auth/login", map[string]any{"username": "admin", "password": "admin123"})
	require.Equal(t, http.StatusOK, login.Code)
	var loginBody struct {
		Token       string   `json:"token"`
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
		Identity    struct {
			Username    string   `json:"username"`
			Role        string   `json:"role"`
			Permissions []string `json:"permissions"`
		} `json:"identity"`
	}
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &loginBody))
	require.NotEmpty(t, tokens["admin"])
	require.Equal(t, "uid:", loginBody.Token[:4])
	require.Equal(t, "admin", loginBody.Role)
	require.Equal(t, loginBody.Permissions, loginBody.Identity.Permissions)
	require.Equal(t, "admin", loginBody.Identity.Username)
	require.Equal(t, "admin", loginBody.Identity.Role)
	require.Contains(t, loginBody.Permissions, "agent:read")
	require.Contains(t, loginBody.Permissions, "rule:publish")
	require.Contains(t, loginBody.Permissions, "activity_reward:publish")

	var admin model.AdminUser
	require.NoError(t, testDB.Where("username = ?", "admin").First(&admin).Error)
	require.NotNil(t, admin.LastLoginAt)

	me := performRequestWithToken(t, router, loginBody.Token, http.MethodGet, "/api/auth/me", nil)
	require.Equal(t, http.StatusOK, me.Code)
	var meBody struct {
		User struct {
			Username    string `json:"username"`
			DisplayName string `json:"displayName"`
			Scope       struct {
				Level string `json:"level"`
			} `json:"scope"`
			Permissions []string `json:"permissions"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(me.Body.Bytes(), &meBody))
	require.Equal(t, "admin", meBody.User.Username)
	require.Equal(t, "platform", meBody.User.Scope.Level)
	require.ElementsMatch(t, loginBody.Permissions, meBody.User.Permissions)

	authorized := performRequestWithToken(t, router, loginBody.Token, http.MethodGet, "/api/orders", nil)
	require.Equal(t, http.StatusOK, authorized.Code)
}

func TestAdminEnumDictionariesUsesConfiguredOverride(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	overridePayload := map[string]any{
		"strict": true,
		"items": []map[string]any{
			{"value": "campaign-special", "label": "活动特场", "labelEn": "Campaign Special"},
		},
	}
	require.NoError(t, testDB.Create(&model.PlatformConfig{
		Key:         enumDictionaryPlatformConfigKey(enumDictionaryActivityTagCode),
		Value:       datatypes.JSON(marshalJSONBytes(overridePayload)),
		Description: "override",
		UpdatedBy:   "test",
	}).Error)

	resp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/enum-dictionaries?codes=activity_tag", nil)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	var body struct {
		Items []enumDictionaryResponse `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, 1, body.Total)
	require.Len(t, body.Items, 1)
	require.Equal(t, enumDictionaryActivityTagCode, body.Items[0].Code)
	require.Len(t, body.Items[0].Items, 1)
	require.Equal(t, "campaign-special", body.Items[0].Items[0].Value)
	require.Equal(t, "活动特场", body.Items[0].Items[0].Label)
}

func TestRBACProtectedRoutePermissionEnforcement(t *testing.T) {

	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)
	seedBindingFixtures(t, testDB)

	cases := []struct {
		name       string
		token      string
		method     string
		path       string
		body       any
		want       int
		forbidOnly bool
	}{
		{name: "operator can read agents", token: "operator", method: http.MethodGet, path: "/api/agents", want: http.StatusOK},
		{name: "operator can read tenants", token: "operator", method: http.MethodGet, path: "/api/tenants", want: http.StatusOK},
		{name: "operator cannot create tenant", token: "operator", method: http.MethodPost, path: "/api/tenants", body: map[string]any{"code": "T-100", "name": "Tenant 100"}, want: http.StatusForbidden},
		{name: "operator can read brands", token: "operator", method: http.MethodGet, path: "/api/brands", want: http.StatusOK},
		{name: "operator cannot create brand", token: "operator", method: http.MethodPost, path: "/api/brands", body: map[string]any{"tenantID": 1, "code": "B-100", "name": "Brand 100"}, want: http.StatusForbidden},
		{name: "operator cannot create agent", token: "operator", method: http.MethodPost, path: "/api/agents", body: map[string]any{"agentNo": "A-100", "name": "Agent 100"}, want: http.StatusForbidden},
		{name: "finance cannot read agents", token: "finance", method: http.MethodGet, path: "/api/agents", want: http.StatusForbidden},
		{name: "finance cannot read tenants", token: "finance", method: http.MethodGet, path: "/api/tenants", want: http.StatusForbidden},
		{name: "finance cannot read brands", token: "finance", method: http.MethodGet, path: "/api/brands", want: http.StatusForbidden},
		{name: "finance can read orders", token: "finance", method: http.MethodGet, path: "/api/orders", want: http.StatusOK},
		{name: "finance can read settlement bills", token: "finance", method: http.MethodGet, path: "/api/settlement-bills", want: http.StatusOK},
		{name: "finance cannot confirm settlement bills", token: "finance", method: http.MethodPost, path: "/api/settlement-bills/1/confirm", want: http.StatusForbidden},
		{name: "finance can export settlement bills", token: "finance", method: http.MethodPost, path: "/api/settlement-bills/1/export", want: http.StatusNotFound},
		{name: "finance can read recalculation tasks", token: "finance", method: http.MethodGet, path: "/api/recalculation-tasks", want: http.StatusOK},
		{name: "finance can create recalculation tasks", token: "finance", method: http.MethodPost, path: "/api/recalculation-tasks", body: map[string]any{"taskType": string(model.RecalculationTaskTypeSettlementBill), "agentID": 1, "periodStart": "2026-04-01T00:00:00Z", "periodEnd": "2026-04-02T00:00:00Z"}, want: http.StatusCreated},
		{name: "finance can execute recharge callback", token: "finance", method: http.MethodPost, path: "/api/recharge/callback", body: minimalRechargeCallbackPayload(), want: http.StatusBadRequest},
		{name: "finance can read risk accounts", token: "finance", method: http.MethodGet, path: "/api/risk/agent-accounts", want: http.StatusOK},
		{name: "finance can create risk cases", token: "finance", method: http.MethodPost, path: "/api/risk/cases", body: map[string]any{"caseNo": "RISK-PERM-1", "agentID": 1, "freeze": false}, want: http.StatusCreated},
		{name: "finance can review risk cases", token: "finance", method: http.MethodPost, path: "/api/risk/cases/RISK-PERM-1/review", body: map[string]any{"action": "confirm"}, want: http.StatusOK},
		{name: "operator cannot create risk cases", token: "operator", method: http.MethodPost, path: "/api/risk/cases", body: map[string]any{"caseNo": "RISK-PERM-2", "agentID": 1, "freeze": false}, want: http.StatusForbidden},
		{name: "operator cannot review risk cases", token: "operator", method: http.MethodPost, path: "/api/risk/cases/RISK-PERM-1/review", body: map[string]any{"action": "confirm"}, want: http.StatusForbidden},
		{name: "finance can read reports", token: "finance", method: http.MethodGet, path: "/api/report/agent-performance", want: http.StatusOK},
		{name: "finance can read team performance report", token: "finance", method: http.MethodGet, path: "/api/report/team-performance", want: http.StatusOK},
		{name: "finance can read settlement progress report", token: "finance", method: http.MethodGet, path: "/api/report/settlement-progress?currency=CNY", want: http.StatusOK},
		{name: "operator cannot execute recharge callback", token: "operator", method: http.MethodPost, path: "/api/recharge/callback", body: minimalRechargeCallbackPayload(), want: http.StatusForbidden},
		{name: "operator can read rules", token: "operator", method: http.MethodGet, path: "/api/rules", want: http.StatusOK},
		{name: "operator can read activity reward rules", token: "operator", method: http.MethodGet, path: "/api/activity-reward-rules", want: http.StatusOK},
		{name: "operator can read activity reward records", token: "operator", method: http.MethodGet, path: "/api/activity-reward-records", want: http.StatusOK},
		{name: "operator cannot create activity reward rule", token: "operator", method: http.MethodPost, path: "/api/activity-reward-rules", body: map[string]any{"ruleName": "Spring Promo", "rewardType": "rebate", "status": "draft", "conditions": []map[string]any{{"field": "depositAmount", "operator": ">=", "value": 100}}, "rewardConfig": map[string]any{"amount": 18}}, want: http.StatusForbidden},
		{name: "operator cannot publish activity reward rule", token: "operator", method: http.MethodPatch, path: "/api/activity-reward-rules/1/status", body: map[string]any{"status": "published"}, want: http.StatusForbidden},
		{name: "finance cannot read activity reward rules", token: "finance", method: http.MethodGet, path: "/api/activity-reward-rules", want: http.StatusForbidden},
		{name: "admin can create activity reward rule", token: "admin", method: http.MethodPost, path: "/api/activity-reward-rules", body: map[string]any{"ruleName": "Admin Promo", "rewardType": "rebate", "status": "draft", "conditions": []map[string]any{{"field": "depositAmount", "operator": ">=", "value": 200}}, "rewardConfig": map[string]any{"amount": 28}}, want: http.StatusCreated, forbidOnly: true},
		{name: "admin can publish activity reward rule", token: "admin", method: http.MethodPatch, path: "/api/activity-reward-rules/1/status", body: map[string]any{"status": "published"}, want: http.StatusOK, forbidOnly: true},
		{name: "operator cannot publish rule", token: "operator", method: http.MethodPost, path: "/api/rules/1/publish", body: map[string]any{}, want: http.StatusForbidden},
		{name: "admin can read audit logs", token: "admin", method: http.MethodGet, path: "/api/audit", want: http.StatusOK},
		{name: "operator can read rbac permissions", token: "operator", method: http.MethodGet, path: "/api/rbac/permissions", want: http.StatusOK},
		{name: "operator can read rbac roles", token: "operator", method: http.MethodGet, path: "/api/rbac/roles", want: http.StatusOK},
		{name: "operator cannot create rbac role", token: "operator", method: http.MethodPost, path: "/api/rbac/roles", body: map[string]any{"code": "tmp", "name": "Tmp"}, want: http.StatusForbidden},
		{name: "finance cannot read audit logs", token: "finance", method: http.MethodGet, path: "/api/audit", want: http.StatusForbidden},
		{name: "finance cannot read rbac permissions", token: "finance", method: http.MethodGet, path: "/api/rbac/permissions", want: http.StatusForbidden},
		{name: "finance cannot read rbac roles", token: "finance", method: http.MethodGet, path: "/api/rbac/roles", want: http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := performMaybeJSONWithToken(t, router, tc.token, tc.method, tc.path, tc.body)
			if tc.forbidOnly {
				require.NotEqual(t, http.StatusForbidden, resp.Code)
				return
			}
			require.Equal(t, tc.want, resp.Code)
			if tc.want == http.StatusForbidden {
				assertJSONErrorBody(t, resp, "forbidden")
			}
		})
	}
}

func TestRBACManagementEndpointsAndAuditLogs(t *testing.T) {

	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	permissionsResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/rbac/permissions", nil)
	require.Equal(t, http.StatusOK, permissionsResp.Code)
	var permissionListResp struct {
		Items []model.AdminPermission `json:"items"`
		Total int                     `json:"total"`
	}
	require.NoError(t, json.Unmarshal(permissionsResp.Body.Bytes(), &permissionListResp))
	require.GreaterOrEqual(t, permissionListResp.Total, 20)
	permissionCodes := make([]string, 0, len(permissionListResp.Items))
	for _, permission := range permissionListResp.Items {
		permissionCodes = append(permissionCodes, permission.Code)
	}
	require.Contains(t, permissionCodes, "activity_reward:read")
	require.Contains(t, permissionCodes, "activity_reward:write")
	require.Contains(t, permissionCodes, "activity_reward:publish")

	rolesResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/rbac/roles", nil)
	require.Equal(t, http.StatusOK, rolesResp.Code)
	var roleListResp struct {
		Items []struct {
			model.AdminRole
			Permissions []string `json:"permissions"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(rolesResp.Body.Bytes(), &roleListResp))
	require.Equal(t, 3, roleListResp.Total)
	require.Contains(t, roleListResp.Items[0].Permissions, "audit:read")

	createRole := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rbac/roles", map[string]any{
		"code":        "reviewer",
		"name":        "Reviewer",
		"permissions": []string{"agent:read", "audit:read"},
	})
	require.Equal(t, http.StatusCreated, createRole.Code)
	var createdRole struct {
		model.AdminRole
		Permissions []string `json:"permissions"`
	}
	require.NoError(t, json.Unmarshal(createRole.Body.Bytes(), &createdRole))
	require.Equal(t, "reviewer", createdRole.Code)
	require.ElementsMatch(t, []string{"agent:read", "audit:read"}, createdRole.Permissions)

	updateRole := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/rbac/roles/"+jsonUint(createdRole.ID), map[string]any{
		"name":        "Senior Reviewer",
		"permissions": []string{"agent:read", "audit:read", "rbac:permissions:view"},
	})
	require.Equal(t, http.StatusOK, updateRole.Code)

	deleteRole := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/rbac/roles/"+jsonUint(createdRole.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteRole.Code)

	usersResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/rbac/users", nil)
	require.Equal(t, http.StatusOK, usersResp.Code, usersResp.Body.String())
	var userListResp struct {
		Items []rbacUserResponse `json:"items"`
		Total int                `json:"total"`
	}
	require.NoError(t, json.Unmarshal(usersResp.Body.Bytes(), &userListResp))
	require.Equal(t, 3, userListResp.Total)
	require.True(t, rbacUserListContainsUsername(userListResp.Items, "admin"))

	createUser := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rbac/users", map[string]any{
		"username":    "auditor",
		"password":    "auditor123",
		"displayName": "Audit User",
		"status":      "active",
		"roles":       []string{"operator"},
	})
	require.Equal(t, http.StatusCreated, createUser.Code, createUser.Body.String())
	var createdUser rbacUserResponse
	require.NoError(t, json.Unmarshal(createUser.Body.Bytes(), &createdUser))
	require.Equal(t, "auditor", createdUser.Username)
	require.ElementsMatch(t, []string{"operator"}, createdUser.Roles)

	updateUser := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/rbac/users/"+jsonUint(createdUser.ID), map[string]any{
		"displayName": "Disabled Auditor",
		"status":      "disabled",
		"roles":       []string{"finance"},
	})
	require.Equal(t, http.StatusOK, updateUser.Code, updateUser.Body.String())
	var updatedUser rbacUserResponse
	require.NoError(t, json.Unmarshal(updateUser.Body.Bytes(), &updatedUser))
	require.Equal(t, model.AdminUserStatusDisabled, updatedUser.Status)
	require.ElementsMatch(t, []string{"finance"}, updatedUser.Roles)

	deleteUser := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/rbac/users/"+jsonUint(createdUser.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteUser.Code)
	recreateUser := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rbac/users", map[string]any{
		"username":    "auditor",
		"password":    "auditor456",
		"displayName": "Audit User Recreated",
		"status":      "active",
		"roles":       []string{"operator"},
	})
	require.Equal(t, http.StatusCreated, recreateUser.Code, recreateUser.Body.String())
	var recreatedUser rbacUserResponse
	require.NoError(t, json.Unmarshal(recreateUser.Body.Bytes(), &recreatedUser))
	require.Equal(t, "auditor", recreatedUser.Username)
	redeleteUser := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/rbac/users/"+jsonUint(recreatedUser.ID), nil)
	require.Equal(t, http.StatusNoContent, redeleteUser.Code)

	tenant := model.Tenant{Code: "rbac-list-tenant", Name: "RBAC List Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	createTenantRole := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rbac/roles", map[string]any{
		"tenantID":    tenant.ID,
		"code":        "tenant_admin_for_list",
		"name":        "Tenant Admin For List",
		"permissions": []string{"rbac:users:view"},
	})
	require.Equal(t, http.StatusCreated, createTenantRole.Code, createTenantRole.Body.String())
	createTenantUser := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rbac/users", map[string]any{
		"tenantID": tenant.ID,
		"username": "tenant-admin-visible",
		"password": "tenant-admin-visible123",
		"status":   "active",
		"roles":    []string{"tenant_admin_for_list"},
	})
	require.Equal(t, http.StatusCreated, createTenantUser.Code, createTenantUser.Body.String())
	usersAfterTenantCreate := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/rbac/users", nil)
	require.Equal(t, http.StatusOK, usersAfterTenantCreate.Code, usersAfterTenantCreate.Body.String())
	var tenantUserList struct {
		Items []rbacUserResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(usersAfterTenantCreate.Body.Bytes(), &tenantUserList))
	require.True(t, rbacUserListContainsUsername(tenantUserList.Items, "tenant-admin-visible"))

	updateTenantUserWithPlatformRole := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/rbac/users/"+jsonUint(findRbacUserIDByUsername(t, tenantUserList.Items, "tenant-admin-visible")), map[string]any{
		"tenantID":    tenant.ID,
		"username":    "tenant-admin-visible",
		"displayName": "Tenant Admin Visible",
		"status":      "active",
		"roles":       []string{"admin"},
	})
	require.Equal(t, http.StatusBadRequest, updateTenantUserWithPlatformRole.Code)
	assertJSONErrorBody(t, updateTenantUserWithPlatformRole, "role not found: admin")

	updateTenantUserWithTenantRole := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/rbac/users/"+jsonUint(findRbacUserIDByUsername(t, tenantUserList.Items, "tenant-admin-visible")), map[string]any{
		"tenantID":    tenant.ID,
		"username":    "tenant-admin-visible",
		"displayName": "Tenant Admin Visible",
		"status":      "active",
		"roles":       []string{"tenant_admin_for_list"},
	})
	require.Equal(t, http.StatusOK, updateTenantUserWithTenantRole.Code, updateTenantUserWithTenantRole.Body.String())

	operatorUsers := performRequestWithToken(t, router, "operator", http.MethodGet, "/api/rbac/users", nil)
	require.Equal(t, http.StatusForbidden, operatorUsers.Code)
	operatorCreateUser := performJSONWithToken(t, router, "operator", http.MethodPost, "/api/rbac/users", map[string]any{"username": "blocked", "password": "blocked123", "roles": []string{"operator"}})
	require.Equal(t, http.StatusForbidden, operatorCreateUser.Code)

	createAgent := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agents", map[string]any{
		"agentNo":     "AG-200",
		"name":        "Admin Created Agent",
		"displayName": "Agent 200",
		"status":      "active",
	})
	require.Equal(t, http.StatusCreated, createAgent.Code)
	var createdAgent model.Agent
	require.NoError(t, json.Unmarshal(createAgent.Body.Bytes(), &createdAgent))
	require.NotZero(t, createdAgent.ID)

	operatorCreate := performJSONWithToken(t, router, "operator", http.MethodPost, "/api/agents", map[string]any{
		"agentNo": "AG-201",
		"name":    "Should Fail",
	})
	require.Equal(t, http.StatusForbidden, operatorCreate.Code)
	assertJSONErrorBody(t, operatorCreate, "forbidden")

	updateStatus := performJSONWithToken(t, router, "admin", http.MethodPatch, "/api/agents/"+jsonUint(createdAgent.ID)+"/status", map[string]any{"status": "inactive"})
	require.Equal(t, http.StatusOK, updateStatus.Code)
	activateStatus := performJSONWithToken(t, router, "admin", http.MethodPatch, "/api/agents/"+jsonUint(createdAgent.ID)+"/status", map[string]any{"status": "active"})
	require.Equal(t, http.StatusOK, activateStatus.Code)

	createInvite := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/invite-codes", map[string]any{
		"agentID": createdAgent.ID,
		"code":    "INV-200",
		"status":  "active",
	})
	require.Equal(t, http.StatusCreated, createInvite.Code)

	operatorInvite := performJSONWithToken(t, router, "operator", http.MethodPost, "/api/invite-codes", map[string]any{
		"agentID": createdAgent.ID,
		"code":    "INV-201",
	})
	require.Equal(t, http.StatusForbidden, operatorInvite.Code)

	readAudit := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/audit", nil)
	require.Equal(t, http.StatusOK, readAudit.Code)
	var auditList struct {
		Items []model.OperationAuditLog `json:"items"`
		Total int                       `json:"total"`
	}
	require.NoError(t, json.Unmarshal(readAudit.Body.Bytes(), &auditList))
	require.GreaterOrEqual(t, auditList.Total, 8)

	var auditRows []model.OperationAuditLog
	require.NoError(t, testDB.Order("id asc").Find(&auditRows).Error)
	require.GreaterOrEqual(t, len(auditRows), 15)
	require.Equal(t, model.AuditModuleAgent, auditRows[0].Module)
	require.Equal(t, "rbac_role_create", auditRows[0].Action)
	require.Equal(t, "admin", auditRows[0].OperatorName)
	require.Equal(t, "admin", auditRows[0].OperatorRole)
	require.Equal(t, model.AuditResultSuccess, auditRows[0].Result)
	require.NotEmpty(t, auditRows[0].AfterPayload)

	require.Equal(t, "rbac_role_update", auditRows[1].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[1].Result)
	require.NotEmpty(t, auditRows[1].BeforePayload)
	require.NotEmpty(t, auditRows[1].AfterPayload)

	require.Equal(t, "rbac_role_delete", auditRows[2].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[2].Result)
	require.NotEmpty(t, auditRows[2].BeforePayload)

	require.Equal(t, "rbac_user_create", auditRows[3].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[3].Result)
	require.NotEmpty(t, auditRows[3].AfterPayload)

	require.Equal(t, "rbac_user_update", auditRows[4].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[4].Result)
	require.NotEmpty(t, auditRows[4].BeforePayload)
	require.NotEmpty(t, auditRows[4].AfterPayload)

	require.Equal(t, "rbac_user_delete", auditRows[5].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[5].Result)
	require.NotEmpty(t, auditRows[5].BeforePayload)

	require.Equal(t, "rbac_user_create", auditRows[6].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[6].Result)
	require.NotEmpty(t, auditRows[6].AfterPayload)

	require.Equal(t, "rbac_user_delete", auditRows[7].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[7].Result)
	require.NotEmpty(t, auditRows[7].BeforePayload)

	require.Equal(t, "rbac_role_create", auditRows[8].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[8].Result)
	require.NotEmpty(t, auditRows[8].AfterPayload)

	require.Equal(t, "rbac_user_create", auditRows[9].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[9].Result)
	require.NotEmpty(t, auditRows[9].AfterPayload)
	require.Equal(t, "rbac_user_update", auditRows[10].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[10].Result)
	require.NotEmpty(t, auditRows[10].BeforePayload)
	require.NotEmpty(t, auditRows[10].AfterPayload)

	require.Equal(t, "create", auditRows[11].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[11].Result)
	require.JSONEq(t, marshalJSON(t, createdAgent), string(auditRows[11].AfterPayload))

	require.Equal(t, "update_status", auditRows[12].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[12].Result)
	require.NotEmpty(t, auditRows[12].BeforePayload)
	require.NotEmpty(t, auditRows[12].AfterPayload)

	require.Equal(t, "update_status", auditRows[13].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[13].Result)
	require.NotEmpty(t, auditRows[13].BeforePayload)
	require.NotEmpty(t, auditRows[13].AfterPayload)

	require.Equal(t, "invite_code_create", auditRows[14].Action)
	require.Equal(t, model.AuditResultSuccess, auditRows[14].Result)
	require.NotEmpty(t, auditRows[14].AfterPayload)
}

func TestPhase3AuditGapIsolationFixes(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "AUD-T1", Name: "Audit Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "AUD-T2", Name: "Audit Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)
	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "AUD-B1", Name: "Audit Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "AUD-B2", Name: "Audit Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)
	scopeTestUser(t, testDB, "admin", &tenantOne.ID, &brandOne.ID)

	agentOne := createAgentFixture(t, testDB, "AG-AUD-1", "Audit Agent 1")
	agentTwo := createAgentFixture(t, testDB, "AG-AUD-2", "Audit Agent 2")
	require.NoError(t, testDB.Model(&agentOne).Updates(map[string]any{"tenant_id": tenantOne.ID, "brand_id": brandOne.ID}).Error)
	require.NoError(t, testDB.Model(&agentTwo).Updates(map[string]any{"tenant_id": tenantTwo.ID, "brand_id": brandTwo.ID}).Error)

	for _, path := range []string{
		"/api/agents/" + jsonUint(agentTwo.ID) + "/ancestors",
		"/api/agents/" + jsonUint(agentTwo.ID) + "/descendants",
		"/api/agents/" + jsonUint(agentTwo.ID) + "/team-stats",
	} {
		resp := performRequestWithToken(t, router, "admin", http.MethodGet, path, nil)
		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())
	}

	inScopeInvite := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/invite-codes", map[string]any{"agentID": agentOne.ID, "code": "INV-AUD-IN"})
	require.Equal(t, http.StatusCreated, inScopeInvite.Code, inScopeInvite.Body.String())
	outOfScopeInvite := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/invite-codes", map[string]any{"agentID": agentTwo.ID, "code": "INV-AUD-OUT"})
	require.Equal(t, http.StatusNotFound, outOfScopeInvite.Code, outOfScopeInvite.Body.String())

	effectiveFrom := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	ruleOne := model.CommissionRule{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, RuleName: "Shared Rule", Scope: model.RuleScopePlatform, RuleType: model.RuleTypeRatio, Status: model.RuleStatusDraft, Version: 1, SettlementRate: 1, EffectiveFrom: effectiveFrom, UniqueKey: buildScopedRuleUniqueKey(&tenantOne.ID, &brandOne.ID, model.RuleScopePlatform, nil, nil, "Shared Rule")}
	ruleTwo := model.CommissionRule{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, RuleName: "Shared Rule", Scope: model.RuleScopePlatform, RuleType: model.RuleTypeRatio, Status: model.RuleStatusPublished, Version: 1, SettlementRate: 1, EffectiveFrom: effectiveFrom, UniqueKey: buildScopedRuleUniqueKey(&tenantTwo.ID, &brandTwo.ID, model.RuleScopePlatform, nil, nil, "Shared Rule")}
	require.NoError(t, testDB.Create(&ruleOne).Error)
	require.NoError(t, testDB.Create(&ruleTwo).Error)

	publishResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rules/"+jsonUint(ruleOne.ID)+"/publish", map[string]any{"publishedBy": "admin"})
	require.Equal(t, http.StatusOK, publishResp.Code, publishResp.Body.String())
	require.NoError(t, testDB.First(&ruleTwo, ruleTwo.ID).Error)
	require.Equal(t, model.RuleStatusPublished, ruleTwo.Status)
}

func TestTenantBrandAdminCanManageScopedRBAC(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenant := model.Tenant{Code: "tenant-rbac", Name: "Tenant RBAC", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "brand-rbac", Name: "Brand RBAC", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brand).Error)
	tenantAdminRole := model.AdminRole{TenantID: &tenant.ID, BrandID: &brand.ID, Code: "tenant_admin", Name: "Tenant Admin"}
	require.NoError(t, testDB.Create(&tenantAdminRole).Error)
	var permissions []model.AdminPermission
	require.NoError(t, testDB.Where("code IN ?", []string{"rbac:permissions:view", "rbac:roles:view", "rbac:roles:write", "rbac:users:view", "rbac:users:write"}).Find(&permissions).Error)
	for _, permission := range permissions {
		require.NoError(t, testDB.Create(&model.AdminRolePermission{RoleID: tenantAdminRole.ID, PermissionID: permission.ID}).Error)
	}
	tenantAdmin := model.AdminUser{Username: "tenant-admin", PasswordHash: "tenant-admin123", DisplayName: "Tenant Admin", Status: model.AdminUserStatusActive}
	require.NoError(t, testDB.Create(&tenantAdmin).Error)
	require.NoError(t, testDB.Create(&model.AdminUserRole{UserID: tenantAdmin.ID, RoleID: tenantAdminRole.ID, TenantID: &tenant.ID, BrandID: &brand.ID}).Error)

	createRole := performJSONWithToken(t, router, "tenant-admin", http.MethodPost, "/api/rbac/roles", map[string]any{"code": "tenant_ops", "name": "Tenant Ops", "permissions": []string{"rbac:permissions:view"}})
	require.Equal(t, http.StatusCreated, createRole.Code, createRole.Body.String())
	var scopedRole rbacRoleResponse
	require.NoError(t, json.Unmarshal(createRole.Body.Bytes(), &scopedRole))
	require.Equal(t, tenant.ID, *scopedRole.TenantID)
	require.Equal(t, brand.ID, *scopedRole.BrandID)

	createUser := performJSONWithToken(t, router, "tenant-admin", http.MethodPost, "/api/rbac/users", map[string]any{"username": "tenant-ops", "password": "tenant-ops123", "status": "active", "roles": []string{"tenant_ops"}})
	require.Equal(t, http.StatusCreated, createUser.Code, createUser.Body.String())
	var scopedUser rbacUserResponse
	require.NoError(t, json.Unmarshal(createUser.Body.Bytes(), &scopedUser))
	require.Equal(t, tenant.ID, *scopedUser.TenantID)
	require.Equal(t, brand.ID, *scopedUser.BrandID)
	duplicateUser := performJSONWithToken(t, router, "tenant-admin", http.MethodPost, "/api/rbac/users", map[string]any{"username": "tenant-ops", "password": "tenant-ops456", "status": "active", "roles": []string{"tenant_ops"}})
	require.Equal(t, http.StatusBadRequest, duplicateUser.Code)
	otherBrand := model.Brand{TenantID: tenant.ID, Code: "brand-rbac-2", Name: "Brand RBAC 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&otherBrand).Error)
	otherBrandRole := model.AdminRole{TenantID: &tenant.ID, BrandID: &otherBrand.ID, Code: "tenant_ops", Name: "Tenant Ops Other Brand"}
	require.NoError(t, testDB.Create(&otherBrandRole).Error)
	for _, permission := range permissions {
		require.NoError(t, testDB.Create(&model.AdminRolePermission{RoleID: otherBrandRole.ID, PermissionID: permission.ID}).Error)
	}
	createSameUsernameOtherBrand := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rbac/users", map[string]any{"username": "tenant-ops", "password": "tenant-ops789", "status": "active", "tenantID": tenant.ID, "brandID": otherBrand.ID, "roles": []string{"tenant_ops"}})
	require.Equal(t, http.StatusCreated, createSameUsernameOtherBrand.Code, createSameUsernameOtherBrand.Body.String())
	var otherBrandUser rbacUserResponse
	require.NoError(t, json.Unmarshal(createSameUsernameOtherBrand.Body.Bytes(), &otherBrandUser))
	require.Equal(t, "tenant-ops", otherBrandUser.Username)
	require.Equal(t, otherBrand.ID, *otherBrandUser.BrandID)

	usersResp := performRequestWithToken(t, router, "tenant-admin", http.MethodGet, "/api/rbac/users", nil)
	require.Equal(t, http.StatusOK, usersResp.Code, usersResp.Body.String())
	var userList struct {
		Items []rbacUserResponse `json:"items"`
	}
	require.NoError(t, json.Unmarshal(usersResp.Body.Bytes(), &userList))
	for _, item := range userList.Items {
		require.Equal(t, tenant.ID, *item.TenantID)
		require.Equal(t, brand.ID, *item.BrandID)
	}

	deleteUser := performRequestWithToken(t, router, "tenant-admin", http.MethodDelete, "/api/rbac/users/"+jsonUint(scopedUser.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteUser.Code)
	deleteOtherBrandUser := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/rbac/users/"+jsonUint(otherBrandUser.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteOtherBrandUser.Code)
	deleteRole := performRequestWithToken(t, router, "tenant-admin", http.MethodDelete, "/api/rbac/roles/"+jsonUint(scopedRole.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteRole.Code)
}

func TestTenantAdminCanManageTenantAndBrandScopedRBAC(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenant := model.Tenant{Code: "tenant-only-rbac", Name: "Tenant Only RBAC", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "tenant-only-brand", Name: "Tenant Only Brand", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brand).Error)
	tenantAdminRole := model.AdminRole{TenantID: &tenant.ID, Code: "tenant_admin", Name: "Tenant Admin"}
	require.NoError(t, testDB.Create(&tenantAdminRole).Error)
	var permissions []model.AdminPermission
	require.NoError(t, testDB.Where("code IN ?", []string{"rbac:permissions:view", "rbac:roles:view", "rbac:roles:write", "rbac:users:view", "rbac:users:write"}).Find(&permissions).Error)
	for _, permission := range permissions {
		require.NoError(t, testDB.Create(&model.AdminRolePermission{RoleID: tenantAdminRole.ID, PermissionID: permission.ID}).Error)
	}
	tenantAdmin := model.AdminUser{Username: "tenant-only-admin", PasswordHash: "tenant-only-admin123", DisplayName: "Tenant Only Admin", Status: model.AdminUserStatusActive}
	require.NoError(t, testDB.Create(&tenantAdmin).Error)
	require.NoError(t, testDB.Create(&model.AdminUserRole{UserID: tenantAdmin.ID, RoleID: tenantAdminRole.ID, TenantID: &tenant.ID}).Error)

	login := performJSONNoAuth(t, router, http.MethodPost, "/api/auth/login", map[string]any{"username": "tenant-only-admin", "password": "tenant-only-admin123"})
	require.Equal(t, http.StatusOK, login.Code, login.Body.String())
	var loginBody authLoginResponse
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &loginBody))
	require.Equal(t, ScopeLevelTenant, loginBody.User.Scope.Level)
	require.Equal(t, tenant.ID, *loginBody.User.Scope.TenantID)
	require.Nil(t, loginBody.User.Scope.BrandID)

	createBrandRole := performJSONWithToken(t, router, "tenant-only-admin", http.MethodPost, "/api/rbac/roles", map[string]any{"tenantID": tenant.ID, "brandID": brand.ID, "code": "brand_ops", "name": "Brand Ops", "permissions": []string{"rbac:permissions:view"}})
	require.Equal(t, http.StatusCreated, createBrandRole.Code, createBrandRole.Body.String())
	var brandRole rbacRoleResponse
	require.NoError(t, json.Unmarshal(createBrandRole.Body.Bytes(), &brandRole))
	require.Equal(t, tenant.ID, *brandRole.TenantID)
	require.Equal(t, brand.ID, *brandRole.BrandID)

	duplicateBrandRole := performJSONWithToken(t, router, "tenant-only-admin", http.MethodPost, "/api/rbac/roles", map[string]any{"tenantID": tenant.ID, "brandID": brand.ID, "code": "brand_ops", "name": "Brand Ops Duplicate", "permissions": []string{"rbac:permissions:view"}})
	require.Equal(t, http.StatusBadRequest, duplicateBrandRole.Code)
	assertJSONErrorBody(t, duplicateBrandRole, "role code already exists in current scope")

	createTenantRoleWithSameCode := performJSONWithToken(t, router, "tenant-only-admin", http.MethodPost, "/api/rbac/roles", map[string]any{"tenantID": tenant.ID, "code": "brand_ops", "name": "Tenant Ops Same Code", "permissions": []string{"rbac:permissions:view"}})
	require.Equal(t, http.StatusCreated, createTenantRoleWithSameCode.Code, createTenantRoleWithSameCode.Body.String())
	var tenantRoleSameCode rbacRoleResponse
	require.NoError(t, json.Unmarshal(createTenantRoleWithSameCode.Body.Bytes(), &tenantRoleSameCode))
	require.Equal(t, tenant.ID, *tenantRoleSameCode.TenantID)
	require.Nil(t, tenantRoleSameCode.BrandID)

	createBrandUser := performJSONWithToken(t, router, "tenant-only-admin", http.MethodPost, "/api/rbac/users", map[string]any{"tenantID": tenant.ID, "brandID": brand.ID, "username": "brand-ops", "password": "brand-ops123", "status": "active", "roles": []string{"brand_ops"}})
	require.Equal(t, http.StatusCreated, createBrandUser.Code, createBrandUser.Body.String())
	var brandUser rbacUserResponse
	require.NoError(t, json.Unmarshal(createBrandUser.Body.Bytes(), &brandUser))
	require.Equal(t, brand.ID, *brandUser.BrandID)
}

func TestAgentAccountLoginIsScopedAtQueryLayer(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	owner := createAgentFixture(t, testDB, "AG-LOGIN-OWNER", "Owner Agent")
	child := createAgentFixture(t, testDB, "AG-LOGIN-CHILD", "Child Agent")
	other := createAgentFixture(t, testDB, "AG-LOGIN-OTHER", "Other Agent")
	createInviteCodeFixture(t, testDB, owner.ID, "INV-LOGIN-OWNER")
	application := applyAndApproveAgentInvite(t, router, child.ID, "INV-LOGIN-OWNER")
	require.NotNil(t, application.ApprovedRelationID)

	role := model.AdminRole{Code: "agent", Name: "Agent Portal User"}
	require.NoError(t, testDB.FirstOrCreate(&role, model.AdminRole{Code: role.Code}).Error)
	permissions := []string{"agent:read", "invite_code:manage", "player:read", "binding:manage", "settlement:read"}
	var permissionRows []model.AdminPermission
	require.NoError(t, testDB.Where("code IN ?", permissions).Find(&permissionRows).Error)
	for _, permission := range permissionRows {
		require.NoError(t, testDB.FirstOrCreate(&model.AdminRolePermission{}, model.AdminRolePermission{RoleID: role.ID, PermissionID: permission.ID}).Error)
	}
	agentUser := model.AdminUser{Username: "agent-owner", PasswordHash: "agent123", DisplayName: "Agent Owner", Status: model.AdminUserStatusActive, AgentID: &owner.ID}
	require.NoError(t, testDB.Create(&agentUser).Error)
	require.NoError(t, testDB.Create(&model.AdminUserRole{UserID: agentUser.ID, RoleID: role.ID}).Error)

	now := time.Now().UTC()
	ownerPlayer := model.Player{PlayerNo: "PLY-OWNER", PlatformUserID: "platform-owner", Currency: "CNY", Status: "active", RegisteredAt: now}
	childPlayer := model.Player{PlayerNo: "PLY-CHILD", PlatformUserID: "platform-child", Currency: "CNY", Status: "active", RegisteredAt: now}
	otherPlayer := model.Player{PlayerNo: "PLY-OTHER", PlatformUserID: "platform-other", Currency: "CNY", Status: "active", RegisteredAt: now}
	require.NoError(t, testDB.Create(&[]model.Player{ownerPlayer, childPlayer, otherPlayer}).Error)
	require.NoError(t, testDB.Where("player_no = ?", ownerPlayer.PlayerNo).First(&ownerPlayer).Error)
	require.NoError(t, testDB.Where("player_no = ?", childPlayer.PlayerNo).First(&childPlayer).Error)
	require.NoError(t, testDB.Where("player_no = ?", otherPlayer.PlayerNo).First(&otherPlayer).Error)
	require.NoError(t, testDB.Create(&[]model.Binding{
		{PlayerID: ownerPlayer.ID, AgentID: owner.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: now, EffectiveFrom: now},
		{PlayerID: childPlayer.ID, AgentID: child.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: now, EffectiveFrom: now},
		{PlayerID: otherPlayer.ID, AgentID: other.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: now, EffectiveFrom: now},
	}).Error)

	require.NoError(t, testDB.Create(&[]model.RechargeOrder{
		{OrderNo: "ORD-OWNER", PlayerID: ownerPlayer.ID, GameID: 1, AgentID: &owner.ID, Amount: 100, PaidAmount: 100, Currency: "CNY", Status: model.OrderStatusPaid, IdempotencyKey: "agent-scope-owner"},
		{OrderNo: "ORD-CHILD", PlayerID: childPlayer.ID, GameID: 1, AgentID: &child.ID, Amount: 100, PaidAmount: 100, Currency: "CNY", Status: model.OrderStatusPaid, IdempotencyKey: "agent-scope-child"},
		{OrderNo: "ORD-OTHER", PlayerID: otherPlayer.ID, GameID: 1, AgentID: &other.ID, Amount: 100, PaidAmount: 100, Currency: "CNY", Status: model.OrderStatusPaid, IdempotencyKey: "agent-scope-other"},
	}).Error)
	require.NoError(t, testDB.Create(&[]model.CommissionRecord{
		{RecordNo: "COM-OWNER", RechargeOrderID: 1, PlayerID: ownerPlayer.ID, AgentID: owner.ID, GameID: 1, CommissionBaseAmount: 100, SettlementRate: 1, CommissionRate: 0.1, CommissionAmount: 10, Currency: "CNY", Status: model.CommissionStatusSettled, EstimatedAt: now},
		{RecordNo: "COM-CHILD", RechargeOrderID: 2, PlayerID: childPlayer.ID, AgentID: child.ID, GameID: 1, CommissionBaseAmount: 100, SettlementRate: 1, CommissionRate: 0.1, CommissionAmount: 10, Currency: "CNY", Status: model.CommissionStatusSettled, EstimatedAt: now},
		{RecordNo: "COM-OTHER", RechargeOrderID: 3, PlayerID: otherPlayer.ID, AgentID: other.ID, GameID: 1, CommissionBaseAmount: 100, SettlementRate: 1, CommissionRate: 0.1, CommissionAmount: 10, Currency: "CNY", Status: model.CommissionStatusSettled, EstimatedAt: now},
	}).Error)

	login := performJSONNoAuth(t, router, http.MethodPost, "/api/auth/login", map[string]any{"username": "agent-owner", "password": "agent123"})
	require.Equal(t, http.StatusOK, login.Code, login.Body.String())
	var loginBody struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &loginBody))

	agentsResp := performRequestWithToken(t, router, loginBody.Token, http.MethodGet, "/api/agents", nil)
	require.Equal(t, http.StatusOK, agentsResp.Code, agentsResp.Body.String())
	var agentsBody listResponse[model.Agent]
	require.NoError(t, json.Unmarshal(agentsResp.Body.Bytes(), &agentsBody))
	require.ElementsMatch(t, []uint64{owner.ID, child.ID}, collectAgentIDs(agentsBody.Items))

	playersResp := performRequestWithToken(t, router, loginBody.Token, http.MethodGet, "/api/players", nil)
	require.Equal(t, http.StatusOK, playersResp.Code, playersResp.Body.String())
	var playersBody listResponse[model.Player]
	require.NoError(t, json.Unmarshal(playersResp.Body.Bytes(), &playersBody))
	require.ElementsMatch(t, []uint64{ownerPlayer.ID, childPlayer.ID}, collectPlayerIDs(playersBody.Items))

	ordersResp := performRequestWithToken(t, router, loginBody.Token, http.MethodGet, "/api/orders", nil)
	require.Equal(t, http.StatusOK, ordersResp.Code, ordersResp.Body.String())
	var ordersBody listResponse[model.RechargeOrder]
	require.NoError(t, json.Unmarshal(ordersResp.Body.Bytes(), &ordersBody))
	require.ElementsMatch(t, []string{"ORD-OWNER", "ORD-CHILD"}, collectOrderNos(ordersBody.Items))

	commissionsResp := performRequestWithToken(t, router, loginBody.Token, http.MethodGet, "/api/commissions", nil)
	require.Equal(t, http.StatusOK, commissionsResp.Code, commissionsResp.Body.String())
	var commissionsBody listResponse[model.CommissionRecord]
	require.NoError(t, json.Unmarshal(commissionsResp.Body.Bytes(), &commissionsBody))
	require.ElementsMatch(t, []string{"COM-OWNER"}, collectCommissionNos(commissionsBody.Items))

	createAgentResp := performJSONWithToken(t, router, loginBody.Token, http.MethodPost, "/api/agents", map[string]any{"agentNo": "AG-BLOCKED", "name": "Blocked"})
	require.Equal(t, http.StatusForbidden, createAgentResp.Code)
}

func TestPhase5SaaSAccountMatrixAcceptance(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "MATRIX-T1", Name: "Matrix Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "MATRIX-T2", Name: "Matrix Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)
	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "MATRIX-B1", Name: "Matrix Brand 1", Status: model.BrandStatusActive}
	brandOneB := model.Brand{TenantID: tenantOne.ID, Code: "MATRIX-B1B", Name: "Matrix Brand 1B", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "MATRIX-B2", Name: "Matrix Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandOneB).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)

	agentOne := model.Agent{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, AgentNo: "MATRIX-AG-1", Name: "Matrix Agent 1", Status: model.AgentStatusActive, Level: 1, Currency: "CNY"}
	agentOneB := model.Agent{TenantID: &tenantOne.ID, BrandID: &brandOneB.ID, AgentNo: "MATRIX-AG-1B", Name: "Matrix Agent 1B", Status: model.AgentStatusActive, Level: 1, Currency: "CNY"}
	agentTwo := model.Agent{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, AgentNo: "MATRIX-AG-2", Name: "Matrix Agent 2", Status: model.AgentStatusActive, Level: 1, Currency: "CNY"}
	require.NoError(t, testDB.Create(&agentOne).Error)
	require.NoError(t, testDB.Create(&agentOneB).Error)
	require.NoError(t, testDB.Create(&agentTwo).Error)

	now := time.Now().UTC()
	playerOne := model.Player{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, PlayerNo: "MATRIX-PLY-1", PlatformUserID: "matrix-player-1", Currency: "CNY", Status: "active", RegisteredAt: now}
	playerOneB := model.Player{TenantID: &tenantOne.ID, BrandID: &brandOneB.ID, PlayerNo: "MATRIX-PLY-1B", PlatformUserID: "matrix-player-1b", Currency: "CNY", Status: "active", RegisteredAt: now}
	playerTwo := model.Player{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, PlayerNo: "MATRIX-PLY-2", PlatformUserID: "matrix-player-2", Currency: "CNY", Status: "active", RegisteredAt: now}
	require.NoError(t, testDB.Create(&playerOne).Error)
	require.NoError(t, testDB.Create(&playerOneB).Error)
	require.NoError(t, testDB.Create(&playerTwo).Error)
	require.NoError(t, testDB.Create(&[]model.Binding{
		{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, PlayerID: playerOne.ID, AgentID: agentOne.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: now, EffectiveFrom: now},
		{TenantID: &tenantOne.ID, BrandID: &brandOneB.ID, PlayerID: playerOneB.ID, AgentID: agentOneB.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: now, EffectiveFrom: now},
		{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, PlayerID: playerTwo.ID, AgentID: agentTwo.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: now, EffectiveFrom: now},
	}).Error)
	require.NoError(t, testDB.Create(&[]model.BindingHistory{
		{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, BindingID: 1, PlayerID: playerOne.ID, ToAgentID: agentOne.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, ChangedAt: now},
		{TenantID: &tenantOne.ID, BrandID: &brandOneB.ID, BindingID: 2, PlayerID: playerOneB.ID, ToAgentID: agentOneB.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, ChangedAt: now},
		{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, BindingID: 3, PlayerID: playerTwo.ID, ToAgentID: agentTwo.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, ChangedAt: now},
	}).Error)
	require.NoError(t, testDB.Create(&[]model.PlatformConfig{
		{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, Key: "matrix.config.one", Value: datatypes.JSON(marshalJSONBytes(map[string]any{"enabled": true})), UpdatedBy: "test"},
		{TenantID: &tenantOne.ID, BrandID: &brandOneB.ID, Key: "matrix.config.one_b", Value: datatypes.JSON(marshalJSONBytes(map[string]any{"enabled": true})), UpdatedBy: "test"},
		{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, Key: "matrix.config.two", Value: datatypes.JSON(marshalJSONBytes(map[string]any{"enabled": true})), UpdatedBy: "test"},
	}).Error)

	createScopedAdminRole(t, testDB, "matrix_tenant_admin", &tenantOne.ID, nil, []string{"tenant:read", "brand:read", "agent:read", "agent:write", "player:read", "binding:manage", "platform_config:read"})
	createScopedAdminUser(t, testDB, "matrix-tenant-admin", "matrix-tenant-admin123", "matrix_tenant_admin", &tenantOne.ID, nil, nil)
	createScopedAdminRole(t, testDB, "matrix_brand_admin", &tenantOne.ID, &brandOne.ID, []string{"tenant:read", "brand:read", "agent:read", "player:read", "binding:manage", "platform_config:read"})
	createScopedAdminUser(t, testDB, "matrix-brand-admin", "matrix-brand-admin123", "matrix_brand_admin", &tenantOne.ID, &brandOne.ID, nil)
	createScopedAdminUser(t, testDB, "matrix-mixed-admin", "matrix-mixed-admin123", "matrix_tenant_admin", &tenantOne.ID, nil, nil)
	var mixedUser model.AdminUser
	require.NoError(t, testDB.Where("username = ?", "matrix-mixed-admin").First(&mixedUser).Error)
	var brandRole model.AdminRole
	require.NoError(t, testDB.Where("code = ?", "matrix_brand_admin").First(&brandRole).Error)
	require.NoError(t, testDB.Create(&model.AdminUserRole{UserID: mixedUser.ID, RoleID: brandRole.ID, TenantID: &tenantOne.ID, BrandID: &brandOne.ID}).Error)
	createScopedAdminRole(t, testDB, "matrix_agent", &tenantOne.ID, &brandOne.ID, []string{"agent:read", "player:read", "binding:manage", "settlement:read"})
	createScopedAdminUser(t, testDB, "matrix-agent", "matrix-agent123", "matrix_agent", &tenantOne.ID, &brandOne.ID, &agentOne.ID)

	tenantLogin := loginAndAssertScope(t, router, "matrix-tenant-admin", "matrix-tenant-admin123", ScopeLevelTenant, &tenantOne.ID, nil, nil)
	brandLogin := loginAndAssertScope(t, router, "matrix-brand-admin", "matrix-brand-admin123", ScopeLevelBrand, &tenantOne.ID, &brandOne.ID, nil)
	mixedLogin := loginAndAssertScope(t, router, "matrix-mixed-admin", "matrix-mixed-admin123", ScopeLevelTenant, &tenantOne.ID, nil, nil)
	agentLogin := loginAndAssertScope(t, router, "matrix-agent", "matrix-agent123", ScopeLevelAgent, &tenantOne.ID, &brandOne.ID, &agentOne.ID)
	adminLogin := loginAndAssertScope(t, router, "admin", "admin123", ScopeLevelPlatform, nil, nil, nil)

	assertAgentListIDs(t, performRequestWithToken(t, router, adminLogin.Token, http.MethodGet, "/api/agents", nil), []uint64{agentOne.ID, agentOneB.ID, agentTwo.ID})
	assertAgentListIDs(t, performRequestWithToken(t, router, tenantLogin.Token, http.MethodGet, "/api/agents", nil), []uint64{agentOne.ID, agentOneB.ID})
	assertAgentListIDs(t, performRequestWithToken(t, router, brandLogin.Token, http.MethodGet, "/api/agents", nil), []uint64{agentOne.ID})
	assertAgentListIDs(t, performRequestWithToken(t, router, mixedLogin.Token, http.MethodGet, "/api/agents", nil), []uint64{agentOne.ID, agentOneB.ID})
	assertAgentListIDs(t, performRequestWithToken(t, router, agentLogin.Token, http.MethodGet, "/api/agents", nil), []uint64{agentOne.ID})

	assertPlayerListIDs(t, performRequestWithToken(t, router, tenantLogin.Token, http.MethodGet, "/api/players", nil), []uint64{playerOne.ID, playerOneB.ID})
	assertPlayerListIDs(t, performRequestWithToken(t, router, brandLogin.Token, http.MethodGet, "/api/players", nil), []uint64{playerOne.ID})
	assertPlayerListIDs(t, performRequestWithToken(t, router, mixedLogin.Token, http.MethodGet, "/api/players", nil), []uint64{playerOne.ID, playerOneB.ID})
	assertPlayerListIDs(t, performRequestWithToken(t, router, agentLogin.Token, http.MethodGet, "/api/players", nil), []uint64{playerOne.ID})
	assertBindingHistoryPlayerIDs(t, performRequestWithToken(t, router, tenantLogin.Token, http.MethodGet, "/api/binding-history", nil), []uint64{playerOne.ID, playerOneB.ID})
	assertPlatformConfigKeys(t, performRequestWithToken(t, router, tenantLogin.Token, http.MethodGet, "/api/platform-configs", nil), []string{"matrix.config.one", "matrix.config.one_b"})
	assertPlatformConfigKeys(t, performRequestWithToken(t, router, brandLogin.Token, http.MethodGet, "/api/platform-configs?brandID="+jsonUint(brandOne.ID), nil), []string{"matrix.config.one"})
	assertPlatformConfigKeys(t, performRequestWithToken(t, router, brandLogin.Token, http.MethodGet, "/api/platform-configs?brandID="+jsonUint(brandTwo.ID), nil), nil)
	assertPlatformConfigKeys(t, performRequestWithToken(t, router, mixedLogin.Token, http.MethodGet, "/api/platform-configs", nil), []string{"matrix.config.one", "matrix.config.one_b"})

	createOutOfScopeAgent := performJSONWithToken(t, router, tenantLogin.Token, http.MethodPost, "/api/agents", map[string]any{"tenantID": tenantTwo.ID, "brandID": brandTwo.ID, "agentNo": "MATRIX-BLOCKED", "name": "Blocked"})
	require.Equal(t, http.StatusNotFound, createOutOfScopeAgent.Code, createOutOfScopeAgent.Body.String())
	createInScopeOtherBrandAgent := performJSONWithToken(t, router, tenantLogin.Token, http.MethodPost, "/api/agents", map[string]any{"tenantID": tenantOne.ID, "brandID": brandOneB.ID, "agentNo": "MATRIX-AG-ALLOW", "name": "Allowed"})
	require.Equal(t, http.StatusCreated, createInScopeOtherBrandAgent.Code, createInScopeOtherBrandAgent.Body.String())
	createAgentByAgentAccount := performJSONWithToken(t, router, agentLogin.Token, http.MethodPost, "/api/agents", map[string]any{"tenantID": tenantOne.ID, "brandID": brandOne.ID, "agentNo": "MATRIX-AGENT-BLOCKED", "name": "Blocked"})
	require.Equal(t, http.StatusForbidden, createAgentByAgentAccount.Code, createAgentByAgentAccount.Body.String())
}

func TestRiskCaseWorkflowAndPlatformConfigPhase3(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenant := model.Tenant{Code: "t1", Name: "Tenant One", DisplayName: "Tenant One", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "b1", Name: "Brand One", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brand).Error)
	agent := model.Agent{AgentNo: "AG-RISK-1", Name: "Risk Agent", DisplayName: "Risk Agent", Status: model.AgentStatusActive, Level: 1, Remark: tenant.Code}
	require.NoError(t, testDB.Create(&agent).Error)
	account := model.AgentAccount{AgentID: agent.ID, Currency: "CNY", Balance: 1000, WithdrawableAmount: 1000}
	require.NoError(t, testDB.Create(&account).Error)

	createRisk := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
		"caseNo":   "RISK-WF-1",
		"agentID":  agent.ID,
		"amount":   200,
		"currency": "CNY",
		"reason":   "suspicious payout",
		"freeze":   true,
		"remark":   "freeze funds",
	})
	require.Equal(t, http.StatusCreated, createRisk.Code)
	var createRiskBody riskCaseResponse
	require.NoError(t, json.Unmarshal(createRisk.Body.Bytes(), &createRiskBody))
	require.Equal(t, "RISK-WF-1", createRiskBody.CaseNo)
	require.Equal(t, string(model.RiskCaseStatusPending), createRiskBody.Status)
	require.True(t, createRiskBody.Frozen)
	require.NotNil(t, createRiskBody.FrozenLedger)
	require.Equal(t, model.LedgerTypeFreeze, createRiskBody.FrozenLedger.LedgerType)

	var createdCase model.RiskCase
	require.NoError(t, testDB.Where("case_no = ?", "RISK-WF-1").First(&createdCase).Error)
	require.Equal(t, model.RiskCaseStatusPending, createdCase.Status)
	require.Equal(t, "finance", createdCase.ReviewedBy)

	reviewRelease := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/RISK-WF-1/review", map[string]any{
		"action": "release",
		"remark": "false positive",
	})
	require.Equal(t, http.StatusOK, reviewRelease.Code)
	var releaseBody riskCaseResponse
	require.NoError(t, json.Unmarshal(reviewRelease.Body.Bytes(), &releaseBody))
	require.Equal(t, string(model.RiskCaseStatusReleased), releaseBody.Status)
	require.NotNil(t, releaseBody.Ledger)
	require.Equal(t, model.LedgerTypeUnfreeze, releaseBody.Ledger.LedgerType)

	require.NoError(t, testDB.Where("case_no = ?", "RISK-WF-1").First(&createdCase).Error)
	require.Equal(t, model.RiskCaseStatusReleased, createdCase.Status)
	require.Equal(t, "false positive", createdCase.ReviewRemark)

	createConfirm := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
		"caseNo":   "RISK-WF-2",
		"agentID":  agent.ID,
		"amount":   120,
		"currency": "CNY",
		"reason":   "manual review",
		"freeze":   true,
	})
	require.Equal(t, http.StatusCreated, createConfirm.Code)

	var confirmCase model.RiskCase
	require.NoError(t, testDB.Where("case_no = ?", "RISK-WF-2").First(&confirmCase).Error)

	reviewConfirm := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/RISK-WF-2/review", map[string]any{
		"action": "confirm",
		"remark": "confirmed risk",
	})
	require.Equal(t, http.StatusOK, reviewConfirm.Code)
	var confirmBody riskCaseResponse
	require.NoError(t, json.Unmarshal(reviewConfirm.Body.Bytes(), &confirmBody))
	require.Equal(t, string(model.RiskCaseStatusConfirmed), confirmBody.Status)
	require.Nil(t, confirmBody.Ledger)

	require.NoError(t, testDB.Where("case_no = ?", "RISK-WF-2").First(&confirmCase).Error)
	require.Equal(t, model.RiskCaseStatusConfirmed, confirmCase.Status)
	require.Equal(t, "confirmed risk", confirmCase.ReviewRemark)

	createPlatformConfig := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/platform-configs", map[string]any{
		"tenantID":    tenant.ID,
		"brandID":     brand.ID,
		"key":         "withdrawal.review.window",
		"value":       map[string]any{"enabled": true, "minutes": 30},
		"description": "phase3 config",
	})
	require.Equal(t, http.StatusCreated, createPlatformConfig.Code)
	var createdConfig model.PlatformConfig
	require.NoError(t, json.Unmarshal(createPlatformConfig.Body.Bytes(), &createdConfig))
	require.Equal(t, "withdrawal.review.window", createdConfig.Key)
	require.Equal(t, "admin", createdConfig.UpdatedBy)

	listScoped := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/platform-configs", nil)
	require.Equal(t, http.StatusOK, listScoped.Code)
	var scopedList struct {
		Items []platformConfigListItem `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listScoped.Body.Bytes(), &scopedList))
	require.Equal(t, 1, scopedList.Total)
	require.Equal(t, tenant.Code, scopedList.Items[0].TenantCode)
	require.Equal(t, brand.Code, scopedList.Items[0].BrandCode)

	listFilteredMiss := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/platform-configs?tenantCode=OTHER", nil)
	require.Equal(t, http.StatusOK, listFilteredMiss.Code)
	var missList struct {
		Items []platformConfigListItem `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listFilteredMiss.Body.Bytes(), &missList))
	require.Zero(t, missList.Total)

	updatePlatformConfig := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/platform-configs/"+jsonUint(createdConfig.ID), map[string]any{
		"tenantID":    tenant.ID,
		"brandID":     brand.ID,
		"key":         "withdrawal.review.window",
		"value":       map[string]any{"enabled": false, "minutes": 45},
		"description": "phase3 config updated",
	})
	require.Equal(t, http.StatusOK, updatePlatformConfig.Code, updatePlatformConfig.Body.String())
	require.NoError(t, json.Unmarshal(updatePlatformConfig.Body.Bytes(), &createdConfig))
	require.Equal(t, "admin", createdConfig.UpdatedBy)
	assert.Contains(t, string(createdConfig.Value), "\"minutes\":45")

	var auditRows []model.OperationAuditLog
	require.NoError(t, testDB.Where("action IN ?", []string{"risk_case_create", "risk_case_review", "platform_config_create", "platform_config_update"}).Order("id asc").Find(&auditRows).Error)
	require.GreaterOrEqual(t, len(auditRows), 5)
}

func TestAuditLogsCaptureFailedMutations(t *testing.T) {

	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)
	seedBindingFixtures(t, testDB)

	resp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/bindings", map[string]any{
		"playerID": 999999,
		"agentID":  1,
	})
	require.Equal(t, http.StatusBadRequest, resp.Code)

	var audits []model.OperationAuditLog
	require.NoError(t, testDB.Order("id asc").Find(&audits).Error)
	require.Len(t, audits, 1)
	require.Equal(t, model.AuditResultFailed, audits[0].Result)
	require.Equal(t, model.AuditModuleBinding, audits[0].Module)
	require.Equal(t, "create", audits[0].Action)
	require.Equal(t, "binding", audits[0].TargetType)
	require.NotEmpty(t, audits[0].AfterPayload)
}

func TestRegisterWithInviteIsIdempotentForSamePlatformUserAndInvite(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-REGISTER", "Register Agent")
	invite := createInviteCodeFixture(t, testDB, agent.ID, "INV-REGISTER")

	payload := map[string]any{
		"playerNo":       "PLY-REGISTER-1",
		"platformUserID": "platform-register-1",
		"nickname":       "Player Register",
		"phone":          "12345678",
		"countryCode":    "US",
		"currency":       "USD",
		"inviteCode":     invite.Code,
		"remark":         "initial register",
	}

	first := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/players/register-with-invite", payload)
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())
	var firstBody struct {
		Player  model.Player  `json:"player"`
		Binding model.Binding `json:"binding"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstBody))
	require.NotZero(t, firstBody.Player.ID)
	require.NotZero(t, firstBody.Binding.ID)

	second := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/players/register-with-invite", payload)
	require.Equal(t, http.StatusCreated, second.Code, second.Body.String())
	var secondBody struct {
		Player  model.Player  `json:"player"`
		Binding model.Binding `json:"binding"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondBody))
	require.Equal(t, firstBody.Player.ID, secondBody.Player.ID)
	require.Equal(t, firstBody.Binding.ID, secondBody.Binding.ID)

	var players []model.Player
	require.NoError(t, testDB.Where("platform_user_id = ?", "platform-register-1").Find(&players).Error)
	require.Len(t, players, 1)

	var bindings []model.Binding
	require.NoError(t, testDB.Where("player_id = ?", firstBody.Player.ID).Find(&bindings).Error)
	require.Len(t, bindings, 1)
	require.Equal(t, model.BindingStatusBound, bindings[0].Status)
	require.NotNil(t, bindings[0].InviteCodeID)
	require.Equal(t, invite.ID, *bindings[0].InviteCodeID)

	var refreshedInvite model.InviteCode
	require.NoError(t, testDB.First(&refreshedInvite, invite.ID).Error)
	require.Equal(t, uint32(1), refreshedInvite.UsedCount)
}

func TestInviteCodeCreationRequiresActiveAgentAndHonorsValidityWindow(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	frozenAgent := createAgentFixture(t, testDB, "AG-FROZEN-INV", "Frozen Invite Agent")
	frozenAgent.Status = model.AgentStatusFrozen
	require.NoError(t, testDB.Save(&frozenAgent).Error)

	createFrozen := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/invite-codes", map[string]any{
		"agentID":   frozenAgent.ID,
		"code":      "INV-FROZEN-DENIED",
		"isPrimary": true,
	})
	require.Equal(t, http.StatusBadRequest, createFrozen.Code, createFrozen.Body.String())
	assertJSONErrorBody(t, createFrozen, "frozen or inactive agent")

	activeAgent := createAgentFixture(t, testDB, "AG-VALID-WINDOW", "Valid Window Agent")
	future := time.Now().UTC().Add(2 * time.Hour)
	validTo := future.Add(24 * time.Hour)
	createFuture := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/invite-codes", map[string]any{
		"agentID":      activeAgent.ID,
		"code":         "INV-FUTURE",
		"status":       "active",
		"validFrom":    future.Format(time.RFC3339),
		"validTo":      validTo.Format(time.RFC3339),
		"channelScope": []string{"ios", "android"},
		"gameScope":    []uint64{101, 202},
	})
	require.Equal(t, http.StatusCreated, createFuture.Code, createFuture.Body.String())

	register := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/players/register-with-invite", map[string]any{
		"playerNo":       "PLY-FUTURE-INV",
		"platformUserID": "platform-future-invite",
		"inviteCode":     "INV-FUTURE",
	})
	require.Equal(t, http.StatusBadRequest, register.Code, register.Body.String())
	assertJSONErrorBody(t, register, "invite code is invalid or unavailable")

	var invite model.InviteCode
	require.NoError(t, testDB.Where("code = ?", "INV-FUTURE").First(&invite).Error)
	require.NotNil(t, invite.ValidFrom)
	require.NotNil(t, invite.ValidTo)
	require.Contains(t, string(invite.ChannelScope), "ios")
	require.Contains(t, string(invite.GameScope), "202")
}

func TestBindingsDuplicateAttemptStillReturnsBadRequest(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-BIND-DUP", "Binding Agent")
	invite := createInviteCodeFixture(t, testDB, agent.ID, "INV-BIND-DUP")
	player := model.Player{
		PlayerNo:       "PLY-BIND-DUP-1",
		PlatformUserID: "platform-bind-dup-1",
		Nickname:       "Binding Player",
		Currency:       "USD",
		Status:         "active",
		RegisteredAt:   time.Now().UTC(),
	}
	require.NoError(t, testDB.Create(&player).Error)

	first := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/bindings", map[string]any{
		"playerID":   player.ID,
		"inviteCode": invite.Code,
		"remark":     "initial binding",
	})
	require.Equal(t, http.StatusCreated, first.Code, first.Body.String())

	duplicate := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/bindings", map[string]any{
		"playerID":   player.ID,
		"inviteCode": invite.Code,
		"remark":     "duplicate binding",
	})
	require.Equal(t, http.StatusBadRequest, duplicate.Code, duplicate.Body.String())
	assertJSONErrorBody(t, duplicate, "already bound")
}

func TestTenantAndBrandManagementEndpoints(t *testing.T) {

	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	listTenants := performRequestWithToken(t, router, "operator", http.MethodGet, "/api/tenants", nil)
	require.Equal(t, http.StatusOK, listTenants.Code)
	var emptyTenants struct {
		Items []model.Tenant `json:"items"`
		Total int            `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listTenants.Body.Bytes(), &emptyTenants))
	require.Zero(t, emptyTenants.Total)

	createTenant := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/tenants", map[string]any{
		"code":        "TENANT-1",
		"name":        "Tenant One",
		"displayName": "Tenant One Display",
	})
	require.Equal(t, http.StatusCreated, createTenant.Code)
	var tenant model.Tenant
	require.NoError(t, json.Unmarshal(createTenant.Body.Bytes(), &tenant))
	require.NotZero(t, tenant.ID)
	require.Equal(t, model.TenantStatusActive, tenant.Status)

	badTenant := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/tenants", map[string]any{
		"name": "Missing Code",
	})
	require.Equal(t, http.StatusBadRequest, badTenant.Code)

	createBrandOne := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/brands", map[string]any{
		"tenantID":  tenant.ID,
		"code":      "BRAND-1",
		"name":      "Brand One",
		"domain":    "brand1.example.com",
		"isDefault": true,
	})
	require.Equal(t, http.StatusCreated, createBrandOne.Code)
	var brandOne model.Brand
	require.NoError(t, json.Unmarshal(createBrandOne.Body.Bytes(), &brandOne))
	require.True(t, brandOne.IsDefault)
	require.Equal(t, model.BrandStatusActive, brandOne.Status)

	createBrandTwo := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/brands", map[string]any{
		"tenantID":  tenant.ID,
		"code":      "BRAND-2",
		"name":      "Brand Two",
		"isDefault": true,
	})
	require.Equal(t, http.StatusCreated, createBrandTwo.Code)
	var brandTwo model.Brand
	require.NoError(t, json.Unmarshal(createBrandTwo.Body.Bytes(), &brandTwo))
	require.True(t, brandTwo.IsDefault)

	missingTenantBrand := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/brands", map[string]any{
		"tenantID": 999999,
		"code":     "BRAND-404",
		"name":     "Brand Missing",
	})
	require.Equal(t, http.StatusBadRequest, missingTenantBrand.Code)

	filteredBrands := performRequestWithToken(t, router, "operator", http.MethodGet, "/api/brands?tenantID="+jsonUint(tenant.ID), nil)
	require.Equal(t, http.StatusOK, filteredBrands.Code)
	var brandList struct {
		Items []model.Brand `json:"items"`
		Total int           `json:"total"`
	}
	require.NoError(t, json.Unmarshal(filteredBrands.Body.Bytes(), &brandList))
	require.Equal(t, 2, brandList.Total)

	updateTenantStatus := performJSONWithToken(t, router, "admin", http.MethodPatch, "/api/tenants/"+jsonUint(tenant.ID)+"/status", map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, updateTenantStatus.Code)

	updateBrandStatus := performJSONWithToken(t, router, "admin", http.MethodPatch, "/api/brands/"+jsonUint(brandTwo.ID)+"/status", map[string]any{"status": "disabled"})
	require.Equal(t, http.StatusOK, updateBrandStatus.Code)

	var refreshedTenant model.Tenant
	require.NoError(t, testDB.First(&refreshedTenant, tenant.ID).Error)
	require.Equal(t, model.TenantStatusDisabled, refreshedTenant.Status)
	require.NotNil(t, refreshedTenant.DefaultBrandID)
	require.Equal(t, brandTwo.ID, *refreshedTenant.DefaultBrandID)

	var persistedBrands []model.Brand
	require.NoError(t, testDB.Where("tenant_id = ?", tenant.ID).Order("id asc").Find(&persistedBrands).Error)
	require.Len(t, persistedBrands, 2)
	require.False(t, persistedBrands[0].IsDefault)
	require.True(t, persistedBrands[1].IsDefault)
	require.Equal(t, model.BrandStatusDisabled, persistedBrands[1].Status)

	var audits []model.OperationAuditLog
	require.NoError(t, testDB.Where("target_type IN ?", []string{"tenant", "brand"}).Order("id asc").Find(&audits).Error)
	require.Len(t, audits, 5)
	require.Equal(t, "tenant_create", audits[0].Action)
	require.Equal(t, "brand_create", audits[1].Action)
	require.Equal(t, "brand_create", audits[2].Action)
	require.Equal(t, "tenant_update_status", audits[3].Action)
	require.Equal(t, "brand_update_status", audits[4].Action)
}

func TestTenantBrandAndGameDeleteSoftDeletesAndAllowsRecreate(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	emptyTenant := model.Tenant{Code: "DEL-TENANT", Name: "Delete Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&emptyTenant).Error)

	deleteTenantResp := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/tenants/"+jsonUint(emptyTenant.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteTenantResp.Code, deleteTenantResp.Body.String())

	require.ErrorIs(t, testDB.First(&model.Tenant{}, emptyTenant.ID).Error, gorm.ErrRecordNotFound)
	var deletedTenant model.Tenant
	require.NoError(t, testDB.Unscoped().First(&deletedTenant, emptyTenant.ID).Error)
	require.True(t, deletedTenant.DeletedAt.Valid)
	require.NotEqual(t, "DEL-TENANT", deletedTenant.Code)
	require.Contains(t, deletedTenant.Code, "__deleted_")

	recreateTenantResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/tenants", map[string]any{"code": "DEL-TENANT", "name": "Delete Tenant Recreated"})
	require.Equal(t, http.StatusCreated, recreateTenantResp.Code, recreateTenantResp.Body.String())

	tenant := model.Tenant{Code: "DEL-SCOPE-T", Name: "Scoped Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "DEL-BRAND", Name: "Delete Brand", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brand).Error)
	game := model.Game{TenantID: &tenant.ID, BrandID: &brand.ID, GameCode: "DEL-GAME", Name: "Delete Game", Status: model.GameStatusOnline, IsAgentable: true, Currency: "CNY"}
	require.NoError(t, testDB.Create(&game).Error)

	deleteGameResp := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/games/"+jsonUint(game.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteGameResp.Code, deleteGameResp.Body.String())

	require.ErrorIs(t, testDB.First(&model.Game{}, game.ID).Error, gorm.ErrRecordNotFound)
	var deletedGame model.Game
	require.NoError(t, testDB.Unscoped().First(&deletedGame, game.ID).Error)
	require.True(t, deletedGame.DeletedAt.Valid)
	require.NotEqual(t, "DEL-GAME", deletedGame.GameCode)
	require.Contains(t, deletedGame.GameCode, "__deleted_")

	recreatedGame := model.Game{TenantID: &tenant.ID, BrandID: &brand.ID, GameCode: "DEL-GAME", Name: "Delete Game Recreated", Status: model.GameStatusDraft, Currency: "CNY"}
	require.NoError(t, testDB.Create(&recreatedGame).Error)

	deleteRecreatedGameResp := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/games/"+jsonUint(recreatedGame.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteRecreatedGameResp.Code, deleteRecreatedGameResp.Body.String())

	deleteBrandResp := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/brands/"+jsonUint(brand.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteBrandResp.Code, deleteBrandResp.Body.String())

	require.ErrorIs(t, testDB.First(&model.Brand{}, brand.ID).Error, gorm.ErrRecordNotFound)
	var deletedBrand model.Brand
	require.NoError(t, testDB.Unscoped().First(&deletedBrand, brand.ID).Error)
	require.True(t, deletedBrand.DeletedAt.Valid)
	require.NotEqual(t, "DEL-BRAND", deletedBrand.Code)
	require.Contains(t, deletedBrand.Code, "__deleted_")

	recreatedBrand := model.Brand{TenantID: tenant.ID, Code: "DEL-BRAND", Name: "Delete Brand Recreated", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&recreatedBrand).Error)
}

func TestDeleteTenantBrandAndGameRespectsScopeAndReferences(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "DEL-SCOPE-T1", Name: "Delete Scope Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "DEL-SCOPE-T2", Name: "Delete Scope Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)

	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "DEL-SCOPE-B1", Name: "Delete Scope Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantOne.ID, Code: "DEL-SCOPE-B2", Name: "Delete Scope Brand 2", Status: model.BrandStatusActive}
	otherBrand := model.Brand{TenantID: tenantTwo.ID, Code: "DEL-SCOPE-B3", Name: "Delete Scope Brand 3", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)
	require.NoError(t, testDB.Create(&otherBrand).Error)

	gameOne := model.Game{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, GameCode: "DEL-SCOPE-G1", Name: "Delete Scope Game 1", Status: model.GameStatusOnline, IsAgentable: true, Currency: "CNY"}
	gameTwo := model.Game{TenantID: &tenantOne.ID, BrandID: &brandTwo.ID, GameCode: "DEL-SCOPE-G2", Name: "Delete Scope Game 2", Status: model.GameStatusOnline, IsAgentable: true, Currency: "CNY"}
	otherGame := model.Game{TenantID: &tenantTwo.ID, BrandID: &otherBrand.ID, GameCode: "DEL-SCOPE-G3", Name: "Delete Scope Game 3", Status: model.GameStatusOnline, IsAgentable: true, Currency: "CNY"}
	require.NoError(t, testDB.Create(&gameOne).Error)
	require.NoError(t, testDB.Create(&gameTwo).Error)
	require.NoError(t, testDB.Create(&otherGame).Error)

	createScopedAdminRole(t, testDB, "tenant_delete_admin", &tenantOne.ID, nil, []string{"tenant:write", "brand:write", "game:write"})
	createScopedAdminUser(t, testDB, "tenant-delete-admin", "tenant-delete-admin123", "tenant_delete_admin", &tenantOne.ID, nil, nil)
	createScopedAdminRole(t, testDB, "brand_delete_admin", &tenantOne.ID, &brandOne.ID, []string{"tenant:write", "brand:write", "game:write"})
	createScopedAdminUser(t, testDB, "brand-delete-admin", "brand-delete-admin123", "brand_delete_admin", &tenantOne.ID, &brandOne.ID, nil)

	tenantLogin := loginAndAssertScope(t, router, "tenant-delete-admin", "tenant-delete-admin123", ScopeLevelTenant, &tenantOne.ID, nil, nil)
	brandLogin := loginAndAssertScope(t, router, "brand-delete-admin", "brand-delete-admin123", ScopeLevelBrand, &tenantOne.ID, &brandOne.ID, nil)

	deleteSameTenantGame := performRequestWithToken(t, router, tenantLogin.Token, http.MethodDelete, "/api/games/"+jsonUint(gameTwo.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteSameTenantGame.Code, deleteSameTenantGame.Body.String())

	deleteSameTenantBrand := performRequestWithToken(t, router, tenantLogin.Token, http.MethodDelete, "/api/brands/"+jsonUint(brandTwo.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteSameTenantBrand.Code, deleteSameTenantBrand.Body.String())
	require.ErrorIs(t, testDB.First(&model.Brand{}, brandTwo.ID).Error, gorm.ErrRecordNotFound)

	deleteTenantByBrand := performRequestWithToken(t, router, brandLogin.Token, http.MethodDelete, "/api/tenants/"+jsonUint(tenantOne.ID), nil)
	require.Equal(t, http.StatusNotFound, deleteTenantByBrand.Code, deleteTenantByBrand.Body.String())
	assertJSONErrorBody(t, deleteTenantByBrand, "record not found")

	deleteOtherBrandGame := performRequestWithToken(t, router, brandLogin.Token, http.MethodDelete, "/api/games/"+jsonUint(otherGame.ID), nil)
	require.Equal(t, http.StatusNotFound, deleteOtherBrandGame.Code, deleteOtherBrandGame.Body.String())
	assertJSONErrorBody(t, deleteOtherBrandGame, "record not found")

	key := model.GameIntegrationKey{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, GameID: gameOne.ID, Name: "delete-scope-key", AccessKey: "ga_live_delete_scope", SecretCiphertext: "gsk_live_delete_scope", Status: model.GameIntegrationKeyStatusActive, Scopes: jsonStringArray(defaultGameOpenAPIScopes())}
	require.NoError(t, testDB.Create(&key).Error)

	deleteReferencedGame := performRequestWithToken(t, router, brandLogin.Token, http.MethodDelete, "/api/games/"+jsonUint(gameOne.ID), nil)
	require.Equal(t, http.StatusBadRequest, deleteReferencedGame.Code, deleteReferencedGame.Body.String())
	assertJSONErrorBody(t, deleteReferencedGame, "game has active integration keys")

	deleteTenantWithBrands := performRequestWithToken(t, router, tenantLogin.Token, http.MethodDelete, "/api/tenants/"+jsonUint(tenantOne.ID), nil)
	require.Equal(t, http.StatusBadRequest, deleteTenantWithBrands.Code, deleteTenantWithBrands.Body.String())
	assertJSONErrorBody(t, deleteTenantWithBrands, "tenant has active brands")
}

func TestTenantScopeHelpers(t *testing.T) {
	t.Run("reads scope from gin context and normalizes values", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set("tenantScope", &TenantScope{
			TenantIDs:   []uint64{0, 8, 8, 9},
			TenantCodes: []string{"  TENANT-A  ", "", "TENANT-A", "TENANT-B"},
			BrandIDs:    []uint64{2, 0, 2, 3},
		})

		scope := ExportedTenantScopeFromContextForTest(ctx)
		require.Equal(t, TenantScope{
			TenantIDs:   []uint64{8, 9},
			TenantCodes: []string{"TENANT-A", "TENANT-B"},
			BrandIDs:    []uint64{2, 3},
		}, scope)
	})

	t.Run("missing scope returns empty scope", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		require.Equal(t, TenantScope{}, ExportedTenantScopeFromContextForTest(ctx))
	})
}

func TestTenantScopeQueryHelpers(t *testing.T) {
	router, testDB := newTestRouter(t)
	_ = router
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "TS-1", Name: "Tenant Scope 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "TS-2", Name: "Tenant Scope 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)

	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "TS-B1", Name: "Tenant Scope Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "TS-B2", Name: "Tenant Scope Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)

	withdrawals := []model.WithdrawalRequest{
		{RequestNo: "WD-TS-1", TenantCode: tenantOne.Code, AgentID: 101, AccountID: 201, Currency: "CNY", Amount: 10, Status: model.WithdrawalStatusPending, IdempotencyKey: "wd-ts-1"},
		{RequestNo: "WD-TS-2", TenantCode: tenantTwo.Code, AgentID: 102, AccountID: 202, Currency: "CNY", Amount: 20, Status: model.WithdrawalStatusPending, IdempotencyKey: "wd-ts-2"},
	}
	require.NoError(t, testDB.Create(&withdrawals).Error)

	var tenantScopedBrands []model.Brand
	require.NoError(t, ExportedScopeByTenantIDForTest(testDB.Model(&model.Brand{}).Order("id asc"), []uint64{0, tenantTwo.ID, tenantTwo.ID}, "tenant_id").Find(&tenantScopedBrands).Error)
	require.Len(t, tenantScopedBrands, 1)
	require.Equal(t, brandTwo.ID, tenantScopedBrands[0].ID)

	var codeScopedWithdrawals []model.WithdrawalRequest
	require.NoError(t, ExportedScopeByTenantCodeForTest(testDB.Model(&model.WithdrawalRequest{}).Order("id asc"), []string{" ", tenantOne.Code, tenantOne.Code}, "tenant_code").Find(&codeScopedWithdrawals).Error)
	require.Len(t, codeScopedWithdrawals, 1)
	require.Equal(t, withdrawals[0].RequestNo, codeScopedWithdrawals[0].RequestNo)

	var brandScopedBrands []model.Brand
	require.NoError(t, ExportedScopeByBrandIDForTest(testDB.Model(&model.Brand{}).Order("id asc"), []uint64{brandOne.ID, brandOne.ID, 0}, "id").Find(&brandScopedBrands).Error)
	require.Len(t, brandScopedBrands, 1)
	require.Equal(t, brandOne.ID, brandScopedBrands[0].ID)

	var unscopedCount int64
	require.NoError(t, ExportedScopeByTenantIDForTest(testDB.Model(&model.Brand{}), nil, "tenant_id").Count(&unscopedCount).Error)
	require.EqualValues(t, 2, unscopedCount)
}

func TestActivityRewardEndpointsRespectTenantScope(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "AR-T1", Name: "Activity Reward Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "AR-T2", Name: "Activity Reward Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)

	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "AR-B1", Name: "Activity Reward Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "AR-B2", Name: "Activity Reward Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)
	scopeTestUser(t, testDB, "admin", &tenantOne.ID, &brandOne.ID)

	ruleOne := model.ActivityRewardRule{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, Name: "Rule In Scope", ActivityType: "deposit", RewardType: "rebate", Status: model.ActivityRewardRuleStatusActive, Currency: "CNY"}
	ruleTwo := model.ActivityRewardRule{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, Name: "Rule Out Of Scope", ActivityType: "deposit", RewardType: "rebate", Status: model.ActivityRewardRuleStatusActive, Currency: "CNY"}
	require.NoError(t, testDB.Create(&ruleOne).Error)
	require.NoError(t, testDB.Create(&ruleTwo).Error)

	recordOne := model.ActivityRewardRecord{RecordNo: "ARR-SCOPE-1", RuleID: ruleOne.ID, TenantID: &tenantOne.ID, BrandID: &brandOne.ID, ActivityType: "deposit", RewardType: "rebate", Status: model.ActivityRewardRecordStatusGranted, RewardValue: 18, Currency: "CNY", ReferenceType: "order", ReferenceID: "AR-REC-1", RuleName: ruleOne.Name}
	recordTwo := model.ActivityRewardRecord{RecordNo: "ARR-SCOPE-2", RuleID: ruleTwo.ID, TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, ActivityType: "deposit", RewardType: "rebate", Status: model.ActivityRewardRecordStatusGranted, RewardValue: 28, Currency: "CNY", ReferenceType: "order", ReferenceID: "AR-REC-2", RuleName: ruleTwo.Name}
	require.NoError(t, testDB.Create(&recordOne).Error)
	require.NoError(t, testDB.Create(&recordTwo).Error)

	t.Run("list ignores cross-tenant tenantID and brandID filters for rules", func(t *testing.T) {
		resp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/activity-reward-rules?tenantID="+jsonUint(tenantTwo.ID)+"&brandID="+jsonUint(brandTwo.ID), nil)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		var payload struct {
			Items []model.ActivityRewardRule `json:"items"`
			Total int                        `json:"total"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.Equal(t, 0, payload.Total)
		require.Empty(t, payload.Items)
	})

	t.Run("list ignores cross-tenant tenantID and brandID filters for records", func(t *testing.T) {
		resp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/activity-reward-records?tenantID="+jsonUint(tenantTwo.ID)+"&brandID="+jsonUint(brandTwo.ID), nil)
		require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
		var payload struct {
			Items []activityRewardRecordView `json:"items"`
			Total int                        `json:"total"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.Equal(t, 0, payload.Total)
		require.Empty(t, payload.Items)
	})
}

func TestActivityRewardRecordLifecycle(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "ARL-T1", Name: "Activity Reward Lifecycle Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "ARL-T2", Name: "Activity Reward Lifecycle Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)

	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "ARL-B1", Name: "Activity Reward Lifecycle Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "ARL-B2", Name: "Activity Reward Lifecycle Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)

	ruleOne := model.ActivityRewardRule{
		TenantID:     &tenantOne.ID,
		BrandID:      &brandOne.ID,
		Name:         "Lifecycle Rule In Scope",
		ActivityType: "deposit",
		RewardType:   "rebate",
		Status:       model.ActivityRewardRuleStatusActive,
		RewardValue:  18,
		Currency:     "CNY",
		DailyLimit:   2,
		TotalLimit:   3,
	}
	ruleTwo := model.ActivityRewardRule{
		TenantID:     &tenantTwo.ID,
		BrandID:      &brandTwo.ID,
		Name:         "Lifecycle Rule Out Of Scope",
		ActivityType: "deposit",
		RewardType:   "rebate",
		Status:       model.ActivityRewardRuleStatusActive,
		RewardValue:  28,
		Currency:     "CNY",
	}
	require.NoError(t, testDB.Create(&ruleOne).Error)
	require.NoError(t, testDB.Create(&ruleTwo).Error)

	t.Run("generate creates immutable record snapshot and list contract fields", func(t *testing.T) {
		resp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules/"+jsonUint(ruleOne.ID)+"/generate", map[string]any{
			"playerID":      1001,
			"agentID":       2001,
			"referenceType": "manual",
			"referenceID":   "ARL-REF-1",
			"remark":        "manual grant",
		})
		require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
		var payload activityRewardRecordView
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
		require.NotZero(t, payload.ID)
		require.NotEmpty(t, payload.RecordNo)
		require.Equal(t, ruleOne.ID, payload.RuleID)
		require.Equal(t, ruleOne.Name, payload.RuleName)
		require.NotNil(t, payload.PlayerID)
		require.EqualValues(t, 1001, *payload.PlayerID)
		require.NotNil(t, payload.UserID)
		require.EqualValues(t, 1001, *payload.UserID)
		require.NotNil(t, payload.AgentID)
		require.EqualValues(t, 2001, *payload.AgentID)
		require.Equal(t, model.ActivityRewardRecordStatusGranted, payload.Status)
		require.Equal(t, "deposit", payload.ActivityType)
		require.Equal(t, "rebate", payload.RewardType)
		require.Equal(t, 18.0, payload.RewardValue)
		require.Equal(t, "CNY", payload.Currency)
		require.Equal(t, "manual grant", payload.Remark)
		require.NotNil(t, payload.GrantedAt)

		var stored model.ActivityRewardRecord
		require.NoError(t, testDB.First(&stored, payload.ID).Error)
		require.Equal(t, ruleOne.Name, stored.RuleName)
		require.NotEmpty(t, stored.RecordNo)
		require.NotEmpty(t, stored.SnapshotPayload)
		var snapshot map[string]any
		require.NoError(t, json.Unmarshal(stored.SnapshotPayload, &snapshot))
		require.Equal(t, ruleOne.Name, snapshot["ruleName"])
		require.Equal(t, "deposit", snapshot["activityType"])
		require.Equal(t, "rebate", snapshot["rewardType"])

		listResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/activity-reward-records?keyword="+payload.RecordNo, nil)
		require.Equal(t, http.StatusOK, listResp.Code, listResp.Body.String())
		var listPayload struct {
			Items []activityRewardRecordView `json:"items"`
			Total int                        `json:"total"`
		}
		require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listPayload))
		require.Equal(t, 1, listPayload.Total)
		require.Len(t, listPayload.Items, 1)
		require.Equal(t, payload.RecordNo, listPayload.Items[0].RecordNo)
		require.Equal(t, payload.RuleName, listPayload.Items[0].RuleName)
	})

	t.Run("generate rejects inactive rule", func(t *testing.T) {
		require.NoError(t, testDB.Model(&ruleOne).Update("status", model.ActivityRewardRuleStatusDraft).Error)
		resp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules/"+jsonUint(ruleOne.ID)+"/generate", map[string]any{
			"playerID":    1002,
			"referenceID": "ARL-REF-INACTIVE",
		})
		require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
		assertJSONErrorBody(t, resp, "activity reward rule is not active")
	})

	t.Run("reverse transitions granted record and blocks duplicate reversal", func(t *testing.T) {
		var stored model.ActivityRewardRecord
		require.NoError(t, testDB.Where("reference_id = ?", "ARL-REF-1").First(&stored).Error)

		reverseResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-records/"+jsonUint(stored.ID)+"/reverse", map[string]any{"remark": "operator reversed"})
		require.Equal(t, http.StatusOK, reverseResp.Code, reverseResp.Body.String())
		var payload activityRewardRecordView
		require.NoError(t, json.Unmarshal(reverseResp.Body.Bytes(), &payload))
		require.Equal(t, model.ActivityRewardRecordStatusReversed, payload.Status)
		require.Equal(t, "operator reversed", payload.Remark)

		var reverseAudit model.OperationAuditLog
		require.NoError(t, testDB.Where("action = ? AND target_id = ?", "activity_reward_record_reverse", jsonUint(stored.ID)).First(&reverseAudit).Error)
		require.Equal(t, "activity_reward_record", reverseAudit.TargetType)
		require.Equal(t, model.AuditResultSuccess, reverseAudit.Result)

		againResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-records/"+jsonUint(stored.ID)+"/reverse", map[string]any{"remark": "again"})
		require.Equal(t, http.StatusBadRequest, againResp.Code, againResp.Body.String())
		assertJSONErrorBody(t, againResp, "activity reward record is not reversible")
	})
}

func TestRuleCreationRejectsInvalidDictionaryValues(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenant := model.Tenant{Code: "RULE-ENUM-T1", Name: "Rule Enum Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brand := model.Brand{TenantID: tenant.ID, Code: "RULE-ENUM-B1", Name: "Rule Enum Brand", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brand).Error)
	game := model.Game{TenantID: &tenant.ID, BrandID: &brand.ID, GameCode: "RULE-ENUM-G1", Name: "Rule Enum Game", Status: model.GameStatusOnline, IsAgentable: true, Currency: "CNY"}
	require.NoError(t, testDB.Create(&game).Error)

	invalidResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rules", map[string]any{
		"tenantID":           tenant.ID,
		"brandID":            brand.ID,
		"ruleName":           "Invalid Enum Rule",
		"scope":              string(model.RuleScopeGame),
		"ruleType":           string(model.RuleTypeRatio),
		"status":             string(model.RuleStatusDraft),
		"priority":           1,
		"version":            1,
		"gameID":             game.ID,
		"maxSettlementDepth": 1,
		"settlementRate":     1,
		"commissionRate":     0.1,
		"fixedAmount":        0,
		"rechargeTypes":      []string{"illegal-type"},
		"activityTags":       []string{"vip"},
		"currency":           "CNY",
		"effectiveFrom":      "2026-04-01T00:00:00Z",
	})
	require.Equal(t, http.StatusBadRequest, invalidResp.Code, invalidResp.Body.String())
	assertJSONErrorBody(t, invalidResp, "invalid rechargeTypes: illegal-type")

	validResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rules", map[string]any{
		"tenantID":           tenant.ID,
		"brandID":            brand.ID,
		"ruleName":           "Valid Enum Rule",
		"scope":              string(model.RuleScopeGame),
		"ruleType":           string(model.RuleTypeRatio),
		"status":             string(model.RuleStatusDraft),
		"priority":           2,
		"version":            1,
		"gameID":             game.ID,
		"maxSettlementDepth": 1,
		"settlementRate":     1,
		"commissionRate":     0.15,
		"fixedAmount":        0,
		"rechargeTypes":      []string{"normal"},
		"activityTags":       []string{"vip"},
		"currency":           "CNY",
		"effectiveFrom":      "2026-04-01T00:00:00Z",
	})
	require.Equal(t, http.StatusCreated, validResp.Code, validResp.Body.String())
}

func TestActivityRewardMutationEndpointsRespectTenantScope(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "ARM-T1", Name: "Activity Reward Mutation Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "ARM-T2", Name: "Activity Reward Mutation Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)

	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "ARM-B1", Name: "Activity Reward Mutation Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "ARM-B2", Name: "Activity Reward Mutation Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)
	scopeTestUser(t, testDB, "admin", &tenantOne.ID, &brandOne.ID)

	inScopeRule := model.ActivityRewardRule{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, Name: "Mutation Rule In Scope", ActivityType: "deposit", RewardType: "rebate", Status: model.ActivityRewardRuleStatusDraft, RewardValue: 12, Currency: "CNY"}
	outOfScopeRule := model.ActivityRewardRule{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, Name: "Mutation Rule Out Of Scope", ActivityType: "deposit", RewardType: "rebate", Status: model.ActivityRewardRuleStatusActive, RewardValue: 22, Currency: "CNY"}
	require.NoError(t, testDB.Create(&inScopeRule).Error)
	require.NoError(t, testDB.Create(&outOfScopeRule).Error)

	outOfScopeRecord := model.ActivityRewardRecord{RecordNo: "ARR-MUT-SCOPE-OUT-1", RuleID: outOfScopeRule.ID, TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, PlayerID: uint64Ptr(1002), AgentID: uint64Ptr(2002), ActivityType: "deposit", RewardType: "rebate", Status: model.ActivityRewardRecordStatusGranted, RewardValue: 22, Currency: "CNY", ReferenceType: "manual", ReferenceID: "ARM-REF-OUT-1", RuleName: outOfScopeRule.Name}
	require.NoError(t, testDB.Create(&outOfScopeRecord).Error)

	t.Run("create rejects out-of-scope tenant and brand", func(t *testing.T) {
		resp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules", map[string]any{
			"tenantID":     tenantTwo.ID,
			"brandID":      brandTwo.ID,
			"name":         "Out Of Scope Create",
			"activityType": "deposit",
			"rewardType":   "rebate",
			"status":       "draft",
			"rewardValue":  18,
			"currency":     "CNY",
			"triggerValue": 100,
			"dailyLimit":   1,
			"totalLimit":   2,
			"remark":       "should be blocked by tenant scope",
		})
		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())
		assertJSONErrorBody(t, resp, "record not found")

		var count int64
		require.NoError(t, testDB.Model(&model.ActivityRewardRule{}).Where("name = ?", "Out Of Scope Create").Count(&count).Error)
		require.EqualValues(t, 0, count)
	})

	t.Run("publish rejects out-of-scope rule", func(t *testing.T) {
		resp := performJSONWithToken(t, router, "admin", http.MethodPatch, "/api/activity-reward-rules/"+jsonUint(outOfScopeRule.ID)+"/status", map[string]any{
			"status": "active",
		})
		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())
		assertJSONErrorBody(t, resp, "record not found")

		var stored model.ActivityRewardRule
		require.NoError(t, testDB.First(&stored, outOfScopeRule.ID).Error)
		require.Equal(t, model.ActivityRewardRuleStatusActive, stored.Status)
	})

	t.Run("generate rejects out-of-scope rule", func(t *testing.T) {
		resp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules/"+jsonUint(outOfScopeRule.ID)+"/generate", map[string]any{
			"playerID":      1002,
			"agentID":       2002,
			"referenceType": "manual",
			"referenceID":   "ARM-GEN-OUT-1",
			"remark":        "should not generate",
		})
		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())
		assertJSONErrorBody(t, resp, "record not found")

		var count int64
		require.NoError(t, testDB.Model(&model.ActivityRewardRecord{}).Where("reference_id = ?", "ARM-GEN-OUT-1").Count(&count).Error)
		require.EqualValues(t, 0, count)
	})

	t.Run("reverse rejects out-of-scope record", func(t *testing.T) {
		resp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-records/"+jsonUint(outOfScopeRecord.ID)+"/reverse", map[string]any{
			"remark": "should not reverse",
		})
		require.Equal(t, http.StatusNotFound, resp.Code, resp.Body.String())
		assertJSONErrorBody(t, resp, "record not found")

		var stored model.ActivityRewardRecord
		require.NoError(t, testDB.First(&stored, outOfScopeRecord.ID).Error)
		require.Equal(t, model.ActivityRewardRecordStatusGranted, stored.Status)
	})

	t.Run("in-scope publish and generate still succeed", func(t *testing.T) {
		publishResp := performJSONWithToken(t, router, "admin", http.MethodPatch, "/api/activity-reward-rules/"+jsonUint(inScopeRule.ID)+"/status", map[string]any{
			"status": "active",
		})
		require.Equal(t, http.StatusOK, publishResp.Code, publishResp.Body.String())

		generateResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules/"+jsonUint(inScopeRule.ID)+"/generate", map[string]any{
			"playerID":      1001,
			"agentID":       2001,
			"referenceType": "manual",
			"referenceID":   "ARM-GEN-IN-1",
			"remark":        "in scope",
		})
		require.Equal(t, http.StatusCreated, generateResp.Code, generateResp.Body.String())
	})
}

func TestActivityRewardLifecycleAuditsUseRuleScope(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "ARA-T1", Name: "Activity Reward Audit Tenant 1", Status: model.TenantStatusActive}
	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "ARA-B1", Name: "Activity Reward Audit Brand 1", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&brandOne).Error)

	rule := model.ActivityRewardRule{
		TenantID:     &tenantOne.ID,
		BrandID:      &brandOne.ID,
		Name:         "Audit Rule In Scope",
		ActivityType: "deposit",
		RewardType:   "rebate",
		Status:       model.ActivityRewardRuleStatusActive,
		RewardValue:  36,
		Currency:     "CNY",
	}
	require.NoError(t, testDB.Create(&rule).Error)

	generateResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules/"+jsonUint(rule.ID)+"/generate", map[string]any{
		"playerID":      3001,
		"agentID":       4001,
		"referenceType": "manual",
		"referenceID":   "ARA-GEN-1",
		"remark":        "audit scope generate",
	})
	require.Equal(t, http.StatusCreated, generateResp.Code, generateResp.Body.String())
	var generated activityRewardRecordView
	require.NoError(t, json.Unmarshal(generateResp.Body.Bytes(), &generated))

	reverseResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-records/"+jsonUint(generated.ID)+"/reverse", map[string]any{
		"remark": "audit scope reverse",
	})
	require.Equal(t, http.StatusOK, reverseResp.Code, reverseResp.Body.String())

	var audits []model.OperationAuditLog
	require.NoError(t, testDB.Where("action IN ?", []string{"activity_reward_record_generate", "activity_reward_record_reverse"}).Order("id asc").Find(&audits).Error)
	require.Len(t, audits, 2)
	for _, audit := range audits {
		require.Equal(t, model.AuditResultSuccess, audit.Result)

		var after struct {
			TenantID uint64 `json:"TenantID"`
			BrandID  uint64 `json:"BrandID"`
		}
		require.NoError(t, json.Unmarshal(audit.AfterPayload, &after))
		require.EqualValues(t, tenantOne.ID, after.TenantID, string(audit.AfterPayload))
		require.EqualValues(t, brandOne.ID, after.BrandID, string(audit.AfterPayload))
	}
}

func TestTenantBootstrapAcceptanceAndScopedIsolation(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	bootstrapTenant := model.Tenant{Code: "P3-BOOT-T1", Name: "Bootstrap Tenant", DisplayName: "Bootstrap Tenant", Status: model.TenantStatusActive}
	outOfScopeTenant := model.Tenant{Code: "P3-BOOT-T2", Name: "Out Of Scope Tenant", DisplayName: "Out Of Scope Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&bootstrapTenant).Error)
	require.NoError(t, testDB.Create(&outOfScopeTenant).Error)

	bootstrapBrand := model.Brand{TenantID: bootstrapTenant.ID, Code: "P3-BOOT-B1", Name: "Bootstrap Brand", DisplayName: "Bootstrap Brand", Status: model.BrandStatusActive, IsDefault: true}
	outOfScopeBrand := model.Brand{TenantID: outOfScopeTenant.ID, Code: "P3-BOOT-B2", Name: "Out Of Scope Brand", DisplayName: "Out Of Scope Brand", Status: model.BrandStatusActive, IsDefault: true}
	require.NoError(t, createBrand(testDB, &bootstrapBrand))
	require.NoError(t, createBrand(testDB, &outOfScopeBrand))
	scopeTestUser(t, testDB, "admin", &bootstrapTenant.ID, &bootstrapBrand.ID)

	bootstrapRule := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules", map[string]any{
		"tenantID":     bootstrapTenant.ID,
		"brandID":      bootstrapBrand.ID,
		"name":         "Bootstrap Reward Rule",
		"activityType": "deposit",
		"rewardType":   "rebate",
		"status":       "draft",
		"rewardValue":  16,
		"currency":     "CNY",
		"triggerValue": 100,
		"dailyLimit":   1,
		"totalLimit":   10,
		"remark":       "tenant bootstrap acceptance",
	})
	require.Equal(t, http.StatusCreated, bootstrapRule.Code, bootstrapRule.Body.String())
	var createdRule model.ActivityRewardRule
	require.NoError(t, json.Unmarshal(bootstrapRule.Body.Bytes(), &createdRule))
	require.NotZero(t, createdRule.ID)
	require.Equal(t, bootstrapTenant.ID, *createdRule.TenantID)
	require.Equal(t, bootstrapBrand.ID, *createdRule.BrandID)

	bootstrapConfig := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/platform-configs", map[string]any{
		"tenantID":    bootstrapTenant.ID,
		"brandID":     bootstrapBrand.ID,
		"key":         "bootstrap.theme",
		"value":       map[string]any{"primaryColor": "#123456", "brandCode": bootstrapBrand.Code},
		"description": "tenant bootstrap config",
	})
	require.Equal(t, http.StatusCreated, bootstrapConfig.Code, bootstrapConfig.Body.String())
	var createdConfig model.PlatformConfig
	require.NoError(t, json.Unmarshal(bootstrapConfig.Body.Bytes(), &createdConfig))
	require.NotZero(t, createdConfig.ID)
	require.NotNil(t, createdConfig.TenantID)
	require.Equal(t, bootstrapTenant.ID, *createdConfig.TenantID)
	require.NotNil(t, createdConfig.BrandID)
	require.Equal(t, bootstrapBrand.ID, *createdConfig.BrandID)

	ruleList := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/activity-reward-rules?tenantID="+jsonUint(bootstrapTenant.ID)+"&brandID="+jsonUint(bootstrapBrand.ID), nil)
	require.Equal(t, http.StatusOK, ruleList.Code, ruleList.Body.String())
	var ruleListBody struct {
		Items []model.ActivityRewardRule `json:"items"`
		Total int                        `json:"total"`
	}
	require.NoError(t, json.Unmarshal(ruleList.Body.Bytes(), &ruleListBody))
	require.Equal(t, 1, ruleListBody.Total)
	require.Len(t, ruleListBody.Items, 1)
	require.Equal(t, createdRule.ID, ruleListBody.Items[0].ID)

	configList := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/platform-configs?tenantCode="+bootstrapTenant.Code, nil)
	require.Equal(t, http.StatusOK, configList.Code, configList.Body.String())
	var configListBody struct {
		Items []platformConfigListItem `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(configList.Body.Bytes(), &configListBody))
	require.Equal(t, 1, configListBody.Total)
	require.Len(t, configListBody.Items, 1)
	require.Equal(t, createdConfig.ID, configListBody.Items[0].ID)
	require.Equal(t, bootstrapTenant.Code, configListBody.Items[0].TenantCode)
	require.Equal(t, bootstrapBrand.Code, configListBody.Items[0].BrandCode)

	outOfScopeRuleCreate := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/activity-reward-rules", map[string]any{
		"tenantID":     outOfScopeTenant.ID,
		"brandID":      outOfScopeBrand.ID,
		"name":         "Out Of Scope Bootstrap Rule",
		"activityType": "deposit",
		"rewardType":   "rebate",
		"status":       "draft",
		"rewardValue":  12,
		"currency":     "CNY",
		"triggerValue": 50,
		"dailyLimit":   1,
		"totalLimit":   5,
	})
	require.Equal(t, http.StatusNotFound, outOfScopeRuleCreate.Code, outOfScopeRuleCreate.Body.String())
	assertJSONErrorBody(t, outOfScopeRuleCreate, "record not found")

	outOfScopeConfigCreate := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/platform-configs", map[string]any{
		"tenantID": outOfScopeTenant.ID,
		"brandID":  outOfScopeBrand.ID,
		"key":      "bootstrap.blocked",
		"value":    map[string]any{"blocked": true},
	})
	require.Equal(t, http.StatusNotFound, outOfScopeConfigCreate.Code, outOfScopeConfigCreate.Body.String())
	assertJSONErrorBody(t, outOfScopeConfigCreate, "record not found")

	outOfScopeRuleRead := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/activity-reward-rules?tenantID="+jsonUint(outOfScopeTenant.ID)+"&brandID="+jsonUint(outOfScopeBrand.ID), nil)
	require.Equal(t, http.StatusOK, outOfScopeRuleRead.Code, outOfScopeRuleRead.Body.String())
	var outOfScopeRuleList struct {
		Items []model.ActivityRewardRule `json:"items"`
		Total int                        `json:"total"`
	}
	require.NoError(t, json.Unmarshal(outOfScopeRuleRead.Body.Bytes(), &outOfScopeRuleList))
	require.Zero(t, outOfScopeRuleList.Total)
	require.Empty(t, outOfScopeRuleList.Items)

	outOfScopeConfigRead := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/platform-configs?tenantCode="+outOfScopeTenant.Code, nil)
	require.Equal(t, http.StatusOK, outOfScopeConfigRead.Code, outOfScopeConfigRead.Body.String())
	var outOfScopeConfigList struct {
		Items []platformConfigListItem `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(outOfScopeConfigRead.Body.Bytes(), &outOfScopeConfigList))
	require.Zero(t, outOfScopeConfigList.Total)
	require.Empty(t, outOfScopeConfigList.Items)

	var blockedRuleCount int64
	require.NoError(t, testDB.Model(&model.ActivityRewardRule{}).Where("name = ?", "Out Of Scope Bootstrap Rule").Count(&blockedRuleCount).Error)
	require.Zero(t, blockedRuleCount)

	var blockedConfigCount int64
	require.NoError(t, testDB.Model(&model.PlatformConfig{}).Where("key = ?", "bootstrap.blocked").Count(&blockedConfigCount).Error)
	require.Zero(t, blockedConfigCount)
}

func TestPhase3TenantScopedEndpoints(t *testing.T) {
	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "P3-T1", Name: "Phase 3 Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "P3-T2", Name: "Phase 3 Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)

	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "P3-B1", Name: "Phase 3 Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "P3-B2", Name: "Phase 3 Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)
	scopeTestUser(t, testDB, "admin", &tenantOne.ID, &brandOne.ID)
	scopeTestUser(t, testDB, "finance", &tenantOne.ID, &brandOne.ID)

	agentOne := createAgentFixture(t, testDB, "AG-P3-1", "Phase 3 Agent 1")
	agentTwo := createAgentFixture(t, testDB, "AG-P3-2", "Phase 3 Agent 2")
	require.NoError(t, testDB.Model(&agentOne).Updates(map[string]any{"tenant_id": tenantOne.ID, "brand_id": brandOne.ID, "remark": tenantOne.Code}).Error)
	require.NoError(t, testDB.Model(&agentTwo).Updates(map[string]any{"tenant_id": tenantTwo.ID, "brand_id": brandTwo.ID, "remark": tenantTwo.Code}).Error)
	require.NoError(t, testDB.First(&agentOne, agentOne.ID).Error)
	require.NoError(t, testDB.First(&agentTwo, agentTwo.ID).Error)

	inviteOne := createInviteCodeFixture(t, testDB, agentOne.ID, "INV-P3-1")
	inviteTwo := createInviteCodeFixture(t, testDB, agentTwo.ID, "INV-P3-2")
	require.NoError(t, testDB.Model(&inviteOne).Updates(map[string]any{"tenant_id": tenantOne.ID, "brand_id": brandOne.ID}).Error)
	require.NoError(t, testDB.Model(&inviteTwo).Updates(map[string]any{"tenant_id": tenantTwo.ID, "brand_id": brandTwo.ID}).Error)
	playerOne := model.Player{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, PlayerNo: "PLY-P3-1", PlatformUserID: "player-p3-1", Currency: "CNY", Status: "active", RegisteredAt: time.Now().UTC()}
	playerTwo := model.Player{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, PlayerNo: "PLY-P3-2", PlatformUserID: "player-p3-2", Currency: "CNY", Status: "active", RegisteredAt: time.Now().UTC()}
	require.NoError(t, testDB.Create(&playerOne).Error)
	require.NoError(t, testDB.Create(&playerTwo).Error)
	gameOne := createGameFixture(t, testDB, "GAME-P3-1", true)
	gameTwo := createGameFixture(t, testDB, "GAME-P3-2", true)
	require.NoError(t, testDB.Model(&gameOne).Updates(map[string]any{"tenant_id": tenantOne.ID, "brand_id": brandOne.ID}).Error)
	require.NoError(t, testDB.Model(&gameTwo).Updates(map[string]any{"tenant_id": tenantTwo.ID, "brand_id": brandTwo.ID}).Error)
	keyOne := model.GameIntegrationKey{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, GameID: gameOne.ID, Name: "p3-key-1", AccessKey: "ga_live_p3_1", SecretCiphertext: "gsk_live_p3_1", Status: model.GameIntegrationKeyStatusActive, Scopes: jsonStringArray(defaultGameOpenAPIScopes())}
	keyTwo := model.GameIntegrationKey{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, GameID: gameTwo.ID, Name: "p3-key-2", AccessKey: "ga_live_p3_2", SecretCiphertext: "gsk_live_p3_2", Status: model.GameIntegrationKeyStatusActive, Scopes: jsonStringArray(defaultGameOpenAPIScopes())}
	require.NoError(t, testDB.Create(&keyOne).Error)
	require.NoError(t, testDB.Create(&keyTwo).Error)
	accessOne := model.AgentGameAccess{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, AgentID: agentOne.ID, GameID: gameOne.ID, Status: model.AccessStatusEnabled, GrantedBy: "seed", GrantedAt: time.Now().UTC(), EffectiveFrom: time.Now().UTC().Add(-time.Hour)}
	accessTwo := model.AgentGameAccess{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, AgentID: agentTwo.ID, GameID: gameTwo.ID, Status: model.AccessStatusEnabled, GrantedBy: "seed", GrantedAt: time.Now().UTC(), EffectiveFrom: time.Now().UTC().Add(-time.Hour)}
	require.NoError(t, testDB.Create(&accessOne).Error)
	require.NoError(t, testDB.Create(&accessTwo).Error)

	accountOne := createAgentAccountFixture(t, testDB, agentOne.ID, "ACC-P3-1")
	accountTwo := createAgentAccountFixture(t, testDB, agentTwo.ID, "ACC-P3-2")
	require.NoError(t, testDB.Model(&accountOne).Updates(map[string]any{"balance": 100, "frozen_balance": 20, "withdrawable_amount": 80}).Error)
	require.NoError(t, testDB.Model(&accountTwo).Updates(map[string]any{"balance": 200, "frozen_balance": 40, "withdrawable_amount": 160}).Error)
	require.NoError(t, testDB.First(&accountOne, accountOne.ID).Error)
	require.NoError(t, testDB.First(&accountTwo, accountTwo.ID).Error)

	billOne := model.SettlementBill{TenantID: &tenantOne.ID, BrandID: &brandOne.ID, BillNo: "SB-P3-1", AgentID: agentOne.ID, PeriodStart: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), Currency: "CNY", Status: model.SettlementBillStatusGenerated}
	billTwo := model.SettlementBill{TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, BillNo: "SB-P3-2", AgentID: agentTwo.ID, PeriodStart: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), PeriodEnd: time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), Currency: "CNY", Status: model.SettlementBillStatusGenerated}
	require.NoError(t, testDB.Create(&billOne).Error)
	require.NoError(t, testDB.Create(&billTwo).Error)

	withdrawalOne := model.WithdrawalRequest{RequestNo: "WD-P3-1", TenantCode: tenantOne.Code, TenantID: &tenantOne.ID, BrandID: &brandOne.ID, AgentID: agentOne.ID, AccountID: accountOne.ID, Currency: "CNY", Amount: 11, Status: model.WithdrawalStatusPending, IdempotencyKey: "wd-p3-1"}
	withdrawalTwo := model.WithdrawalRequest{RequestNo: "WD-P3-2", TenantCode: tenantTwo.Code, TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, AgentID: agentTwo.ID, AccountID: accountTwo.ID, Currency: "CNY", Amount: 22, Status: model.WithdrawalStatusPending, IdempotencyKey: "wd-p3-2"}
	require.NoError(t, testDB.Create(&withdrawalOne).Error)
	require.NoError(t, testDB.Create(&withdrawalTwo).Error)

	ledgerOne := model.AgentAccountLedger{AgentID: agentOne.ID, ReferenceType: "risk_case", ReferenceID: "RISK-P3-1", LedgerType: model.LedgerTypeFreeze, Amount: 5, Currency: "CNY", IdempotencyKey: "risk-p3-1"}
	ledgerTwo := model.AgentAccountLedger{AgentID: agentTwo.ID, ReferenceType: "risk_case", ReferenceID: "RISK-P3-2", LedgerType: model.LedgerTypeFreeze, Amount: 6, Currency: "CNY", IdempotencyKey: "risk-p3-2"}
	require.NoError(t, testDB.Create(&ledgerOne).Error)
	require.NoError(t, testDB.Create(&ledgerTwo).Error)

	riskCaseOne := model.RiskCase{CaseNo: "RISK-P3-1", TenantID: &tenantOne.ID, BrandID: &brandOne.ID, AgentID: agentOne.ID, Amount: 5, Currency: "CNY", Status: model.RiskCaseStatusPending, FreezeRequested: true}
	riskCaseTwo := model.RiskCase{CaseNo: "RISK-P3-2", TenantID: &tenantTwo.ID, BrandID: &brandTwo.ID, AgentID: agentTwo.ID, Amount: 6, Currency: "CNY", Status: model.RiskCaseStatusPending, FreezeRequested: true}
	require.NoError(t, testDB.Create(&riskCaseOne).Error)
	require.NoError(t, testDB.Create(&riskCaseTwo).Error)

	t.Run("tenant scoped list endpoints only return in-scope records", func(t *testing.T) {
		gamesResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/games", nil)
		require.Equal(t, http.StatusOK, gamesResp.Code, gamesResp.Body.String())
		var gameList struct {
			Items []model.Game `json:"items"`
			Total int          `json:"total"`
		}
		require.NoError(t, json.Unmarshal(gamesResp.Body.Bytes(), &gameList))
		require.Equal(t, 1, gameList.Total)
		require.Len(t, gameList.Items, 1)
		require.Equal(t, gameOne.ID, gameList.Items[0].ID)

		gameKeysResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/games/"+jsonUint(gameOne.ID)+"/integration-keys", nil)
		require.Equal(t, http.StatusOK, gameKeysResp.Code, gameKeysResp.Body.String())
		var gameKeyList struct {
			Items []gameCredentialResponse `json:"items"`
			Total int                      `json:"total"`
		}
		require.NoError(t, json.Unmarshal(gameKeysResp.Body.Bytes(), &gameKeyList))
		require.Equal(t, 1, gameKeyList.Total)
		require.Len(t, gameKeyList.Items, 1)
		require.Equal(t, keyOne.ID, gameKeyList.Items[0].ID)
		require.Equal(t, gameOne.ID, gameKeyList.Items[0].GameID)

		outOfScopeKeysResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/games/"+jsonUint(gameTwo.ID)+"/integration-keys", nil)
		require.Equal(t, http.StatusNotFound, outOfScopeKeysResp.Code, outOfScopeKeysResp.Body.String())

		accessResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agent-game-access", nil)
		require.Equal(t, http.StatusOK, accessResp.Code, accessResp.Body.String())
		var accessList struct {
			Items []agentGameAccessListItem `json:"items"`
			Total int                       `json:"total"`
		}
		require.NoError(t, json.Unmarshal(accessResp.Body.Bytes(), &accessList))
		require.Equal(t, 1, accessList.Total)
		require.Len(t, accessList.Items, 1)
		require.Equal(t, accessOne.ID, accessList.Items[0].ID)
		require.Equal(t, gameOne.GameCode, accessList.Items[0].GameCode)

		settlementResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/settlement-bills", nil)
		require.Equal(t, http.StatusOK, settlementResp.Code, settlementResp.Body.String())
		var settlementList struct {
			Items []settlementBillListItem `json:"items"`
			Total int                      `json:"total"`
		}
		require.NoError(t, json.Unmarshal(settlementResp.Body.Bytes(), &settlementList))
		require.Equal(t, 1, settlementList.Total)
		require.Len(t, settlementList.Items, 1)
		require.Equal(t, billOne.ID, settlementList.Items[0].ID)
		require.Equal(t, billOne.BillNo, settlementList.Items[0].BillNo)
		require.Equal(t, agentOne.ID, settlementList.Items[0].AgentID)

		withdrawalResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/withdrawal-requests", nil)
		require.Equal(t, http.StatusOK, withdrawalResp.Code, withdrawalResp.Body.String())
		var withdrawalList struct {
			Items []withdrawalListItem `json:"items"`
			Total int                  `json:"total"`
		}
		require.NoError(t, json.Unmarshal(withdrawalResp.Body.Bytes(), &withdrawalList))
		require.Equal(t, 1, withdrawalList.Total)
		require.Len(t, withdrawalList.Items, 1)
		require.Equal(t, withdrawalOne.ID, withdrawalList.Items[0].ID)
		require.Equal(t, withdrawalOne.RequestNo, withdrawalList.Items[0].RequestNo)
		require.Equal(t, tenantOne.Code, withdrawalList.Items[0].TenantCode)
		require.Equal(t, agentOne.ID, withdrawalList.Items[0].AgentID)
		require.Equal(t, agentOne.Name, withdrawalList.Items[0].AgentName)

		riskResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/risk/agent-accounts", nil)
		require.Equal(t, http.StatusOK, riskResp.Code, riskResp.Body.String())
		var riskList struct {
			Items []agentAccountRiskListItem `json:"items"`
			Total int                        `json:"total"`
		}
		require.NoError(t, json.Unmarshal(riskResp.Body.Bytes(), &riskList))
		require.Equal(t, 1, riskList.Total)
		require.Len(t, riskList.Items, 1)
		require.Equal(t, accountOne.ID, riskList.Items[0].ID)
		require.Equal(t, agentOne.ID, riskList.Items[0].AgentID)
		require.Equal(t, accountOne.Balance, riskList.Items[0].Balance)
		require.Equal(t, accountOne.FrozenBalance, riskList.Items[0].FrozenBalance)
		require.Equal(t, accountOne.WithdrawableAmount, riskList.Items[0].WithdrawableAmount)

		riskCasesResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/risk/cases?status=pending&riskLevel=medium&minFrozenRatio=0.2", nil)
		require.Equal(t, http.StatusOK, riskCasesResp.Code, riskCasesResp.Body.String())
		var riskCaseList struct {
			Items []riskCaseListItem `json:"items"`
			Total int                `json:"total"`
		}
		require.NoError(t, json.Unmarshal(riskCasesResp.Body.Bytes(), &riskCaseList))
		require.Equal(t, 1, riskCaseList.Total)
		require.Len(t, riskCaseList.Items, 1)
		require.Equal(t, riskCaseOne.CaseNo, riskCaseList.Items[0].CaseNo)
		require.Equal(t, agentOne.ID, riskCaseList.Items[0].AgentID)
		require.Equal(t, agentOne.Name, riskCaseList.Items[0].AgentName)
		require.Equal(t, "medium", riskCaseList.Items[0].RiskLevel)
		require.Equal(t, string(model.RiskCaseStatusPending), riskCaseList.Items[0].Status)
		require.True(t, riskCaseList.Items[0].FreezeRequested)
		require.InDelta(t, 20.0, riskCaseList.Items[0].FrozenBalance, 0.001)
		require.InDelta(t, 80.0, riskCaseList.Items[0].WithdrawableAmount, 0.001)
		require.InDelta(t, 0.2, riskCaseList.Items[0].FrozenRatio, 0.001)
		require.NotNil(t, riskCaseList.Items[0].LatestWithdrawalRequestID)
		require.Equal(t, withdrawalOne.RequestNo, riskCaseList.Items[0].LatestWithdrawalRequestNo)
		require.NotNil(t, riskCaseList.Items[0].LatestWithdrawalAmount)
		require.InDelta(t, withdrawalOne.Amount, *riskCaseList.Items[0].LatestWithdrawalAmount, 0.001)

		reportResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/agent-performance", nil)
		require.Equal(t, http.StatusOK, reportResp.Code, reportResp.Body.String())
		var reportList struct {
			Items []agentPerformanceReportItem `json:"items"`
			Total int                          `json:"total"`
		}
		require.NoError(t, json.Unmarshal(reportResp.Body.Bytes(), &reportList))
		require.Equal(t, 1, reportList.Total)
		require.Len(t, reportList.Items, 1)
		require.Equal(t, agentOne.ID, reportList.Items[0].AgentID)
		require.Equal(t, agentOne.Name, reportList.Items[0].AgentName)
		require.Equal(t, accountOne.Balance, reportList.Items[0].Balance)
		require.Equal(t, accountOne.FrozenBalance, reportList.Items[0].FrozenBalance)
		require.Equal(t, accountOne.WithdrawableAmount, reportList.Items[0].WithdrawableAmount)
		require.EqualValues(t, 1, reportList.Items[0].TotalEntries)
		require.Equal(t, 5.0, reportList.Items[0].FreezeAmount)
		require.Equal(t, 5.0, reportList.Items[0].DebitAmount)

		gameSettlementResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/game-settlement?currency=CNY", nil)
		require.Equal(t, http.StatusOK, gameSettlementResp.Code, gameSettlementResp.Body.String())
		var gameSettlementList struct {
			Items []gameSettlementReportItem `json:"items"`
			Total int                        `json:"total"`
		}
		require.NoError(t, json.Unmarshal(gameSettlementResp.Body.Bytes(), &gameSettlementList))
		require.Equal(t, 1, gameSettlementList.Total)
		require.Len(t, gameSettlementList.Items, 1)
		require.Equal(t, "CNY", gameSettlementList.Items[0].Currency)
		require.EqualValues(t, 1, gameSettlementList.Items[0].TotalBills)

		teamResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/team-performance", nil)
		require.Equal(t, http.StatusOK, teamResp.Code, teamResp.Body.String())
		var teamList struct {
			Items []teamPerformanceReportItem `json:"items"`
			Total int                         `json:"total"`
		}
		require.NoError(t, json.Unmarshal(teamResp.Body.Bytes(), &teamList))
		require.Equal(t, 1, teamList.Total)
		require.Len(t, teamList.Items, 1)
		require.Equal(t, agentOne.ID, teamList.Items[0].AgentID)

		progressResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/settlement-progress?currency=CNY", nil)
		require.Equal(t, http.StatusOK, progressResp.Code, progressResp.Body.String())
		var progressList struct {
			Items []settlementProgressReportItem `json:"items"`
			Total int                            `json:"total"`
		}
		require.NoError(t, json.Unmarshal(progressResp.Body.Bytes(), &progressList))
		require.Equal(t, 1, progressList.Total)
		require.Len(t, progressList.Items, 1)
		require.Equal(t, "CNY", progressList.Items[0].Currency)
		require.EqualValues(t, 1, progressList.Items[0].TotalBills)

		dataLayersResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/data-platform-layers", nil)
		require.Equal(t, http.StatusOK, dataLayersResp.Code, dataLayersResp.Body.String())
		var dataLayerList struct {
			Items []dataPlatformLayerItem `json:"items"`
			Total int                     `json:"total"`
		}
		require.NoError(t, json.Unmarshal(dataLayersResp.Body.Bytes(), &dataLayerList))
		require.Equal(t, 4, dataLayerList.Total)
		require.Len(t, dataLayerList.Items, 4)
		require.Equal(t, "ODS", dataLayerList.Items[0].Layer)
		require.Equal(t, "ready", dataLayerList.Items[0].Status)
		require.GreaterOrEqual(t, dataLayerList.Items[1].RecordCount, int64(1))

		dataMetricsResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/data-platform-metrics", nil)
		require.Equal(t, http.StatusOK, dataMetricsResp.Code, dataMetricsResp.Body.String())
		var dataMetricList struct {
			Items []dataPlatformMetricItem `json:"items"`
			Total int                      `json:"total"`
		}
		require.NoError(t, json.Unmarshal(dataMetricsResp.Body.Bytes(), &dataMetricList))
		require.Equal(t, 10, dataMetricList.Total)
		require.Len(t, dataMetricList.Items, 10)
		metricByKey := make(map[string]dataPlatformMetricItem)
		for _, item := range dataMetricList.Items {
			metricByKey[item.Key] = item
		}
		require.Contains(t, metricByKey, "profitDataCoverage")
		require.Contains(t, metricByKey, "withdrawalCost")
		require.Contains(t, metricByKey, "withdrawalSuccessRate")
		require.Contains(t, metricByKey, "riskInterceptRate")
		require.Equal(t, "percent", metricByKey["riskInterceptRate"].Unit)

		intelligenceResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/risk/intelligence", nil)
		require.Equal(t, http.StatusOK, intelligenceResp.Code, intelligenceResp.Body.String())
		var intelligenceList struct {
			Items []riskIntelligenceItem `json:"items"`
			Total int                    `json:"total"`
		}
		require.NoError(t, json.Unmarshal(intelligenceResp.Body.Bytes(), &intelligenceList))
		require.GreaterOrEqual(t, intelligenceList.Total, 1)
		require.NotEmpty(t, intelligenceList.Items)
		require.Equal(t, "risk_case", intelligenceList.Items[0].SourceType)
		require.Equal(t, riskCaseOne.CaseNo, intelligenceList.Items[0].SourceID)
		require.Equal(t, agentOne.ID, intelligenceList.Items[0].AgentID)
		require.True(t, intelligenceList.Items[0].Intercepted)
		require.GreaterOrEqual(t, intelligenceList.Items[0].Score, 70.0)
	})

	t.Run("tenant scoped business writes reject out of scope references", func(t *testing.T) {
		outOfScopeRegister := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/players/register-with-invite", map[string]any{"platformUserID": "player-p3-blocked", "inviteCode": inviteTwo.Code})
		require.Equal(t, http.StatusBadRequest, outOfScopeRegister.Code, outOfScopeRegister.Body.String())

		outOfScopeBinding := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/bindings", map[string]any{"playerID": playerOne.ID, "code": inviteTwo.Code})
		require.Equal(t, http.StatusBadRequest, outOfScopeBinding.Code, outOfScopeBinding.Body.String())

		outOfScopeInviteApply := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agent-invite-applications", map[string]any{"applicantAgentID": agentTwo.ID, "inviteCode": inviteOne.Code})
		require.Equal(t, http.StatusNotFound, outOfScopeInviteApply.Code, outOfScopeInviteApply.Body.String())

		outOfScopeLedger := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{"agentID": agentTwo.ID, "referenceType": "manual", "referenceID": "P3-OUT", "ledgerType": string(model.LedgerTypeIncome), "amount": 1, "idempotencyKey": "ledger-p3-out"})
		require.Equal(t, http.StatusNotFound, outOfScopeLedger.Code, outOfScopeLedger.Body.String())

		outOfScopeTask := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{"taskType": string(model.RecalculationTaskTypeSettlementBill), "agentID": agentTwo.ID, "periodStart": "2026-04-01T00:00:00Z", "periodEnd": "2026-04-02T00:00:00Z"})
		require.Equal(t, http.StatusNotFound, outOfScopeTask.Code, outOfScopeTask.Body.String())

		outOfScopeRule := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rules", map[string]any{"tenantID": tenantOne.ID, "brandID": brandOne.ID, "ruleName": "P3 Out Rule", "scope": string(model.RuleScopeAgent), "agentID": agentTwo.ID, "ruleType": string(model.RuleTypeRatio), "settlementRate": 1, "effectiveFrom": "2026-04-01T00:00:00Z"})
		require.Equal(t, http.StatusNotFound, outOfScopeRule.Code, outOfScopeRule.Body.String())

		inScopeLedger := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{"agentID": agentOne.ID, "referenceType": "manual", "referenceID": "P3-IN", "ledgerType": string(model.LedgerTypeIncome), "amount": 1, "idempotencyKey": "ledger-p3-in"})
		require.Equal(t, http.StatusCreated, inScopeLedger.Code, inScopeLedger.Body.String())

		inScopeRule := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/rules", map[string]any{"tenantID": tenantOne.ID, "brandID": brandOne.ID, "ruleName": "P3 In Rule", "scope": string(model.RuleScopeGame), "gameID": gameOne.ID, "ruleType": string(model.RuleTypeRatio), "settlementRate": 1, "effectiveFrom": "2026-04-01T00:00:00Z"})
		require.Equal(t, http.StatusCreated, inScopeRule.Code, inScopeRule.Body.String())
	})

	t.Run("tenant scoped write actions reject out of scope records", func(t *testing.T) {
		updateGameResp := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/games/"+jsonUint(gameTwo.ID), map[string]any{"name": "blocked"})
		require.Equal(t, http.StatusNotFound, updateGameResp.Code, updateGameResp.Body.String())
		assertJSONErrorBody(t, updateGameResp, "record not found")

		publishGameResp := performJSONWithToken(t, router, "admin", http.MethodPatch, "/api/games/"+jsonUint(gameTwo.ID)+"/status", map[string]any{"status": string(model.GameStatusOffline)})
		require.Equal(t, http.StatusNotFound, publishGameResp.Code, publishGameResp.Body.String())
		assertJSONErrorBody(t, publishGameResp, "record not found")

		rotateKeyResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/games/"+jsonUint(gameTwo.ID)+"/integration-keys/"+jsonUint(keyTwo.ID)+"/rotate", map[string]any{})
		require.Equal(t, http.StatusNotFound, rotateKeyResp.Code, rotateKeyResp.Body.String())
		assertJSONErrorBody(t, rotateKeyResp, "record not found")

		revokeAccessResp := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/agent-game-access?agentID="+jsonUint(agentTwo.ID)+"&gameID="+jsonUint(gameTwo.ID), nil)
		require.Equal(t, http.StatusNotFound, revokeAccessResp.Code, revokeAccessResp.Body.String())
		assertJSONErrorBody(t, revokeAccessResp, "record not found")

		confirmResp := performRequestWithToken(t, router, "admin", http.MethodPost, "/api/settlement-bills/"+jsonUint(billTwo.ID)+"/confirm", nil)
		require.Equal(t, http.StatusNotFound, confirmResp.Code, confirmResp.Body.String())
		assertJSONErrorBody(t, confirmResp, "record not found")

		reviewResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawal-requests/"+jsonUint(withdrawalTwo.ID)+"/review", map[string]any{"action": "approve"})
		require.Equal(t, http.StatusNotFound, reviewResp.Code, reviewResp.Body.String())
		assertJSONErrorBody(t, reviewResp, "record not found")
	})

	t.Run("tenant scoped risk case flows reject out of scope cases and keep in-scope writable", func(t *testing.T) {
		outOfScopeReview := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/"+riskCaseTwo.CaseNo+"/review", map[string]any{"action": "confirm"})
		require.Equal(t, http.StatusNotFound, outOfScopeReview.Code, outOfScopeReview.Body.String())
		assertJSONErrorBody(t, outOfScopeReview, "record not found")

		inScopeReview := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/"+riskCaseOne.CaseNo+"/review", map[string]any{"action": "confirm", "remark": "confirmed in scope"})
		require.Equal(t, http.StatusOK, inScopeReview.Code, inScopeReview.Body.String())

		var storedInScope model.RiskCase
		require.NoError(t, testDB.Where("case_no = ?", riskCaseOne.CaseNo).First(&storedInScope).Error)
		require.Equal(t, model.RiskCaseStatusConfirmed, storedInScope.Status)
		require.Equal(t, "confirmed in scope", storedInScope.ReviewRemark)

		var storedOutOfScope model.RiskCase
		require.NoError(t, testDB.Where("case_no = ?", riskCaseTwo.CaseNo).First(&storedOutOfScope).Error)
		require.Equal(t, model.RiskCaseStatusPending, storedOutOfScope.Status)

		confirmedListResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/risk/cases?status=confirmed", nil)
		require.Equal(t, http.StatusOK, confirmedListResp.Code, confirmedListResp.Body.String())
		var confirmedRiskCaseList struct {
			Items []riskCaseListItem `json:"items"`
			Total int                `json:"total"`
		}
		require.NoError(t, json.Unmarshal(confirmedListResp.Body.Bytes(), &confirmedRiskCaseList))
		require.Equal(t, 1, confirmedRiskCaseList.Total)
		require.Len(t, confirmedRiskCaseList.Items, 1)
		require.Equal(t, riskCaseOne.CaseNo, confirmedRiskCaseList.Items[0].CaseNo)
		require.Equal(t, string(model.RiskCaseStatusConfirmed), confirmedRiskCaseList.Items[0].Status)

		createOutOfScope := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
			"caseNo":   "RISK-P3-CREATE-OUT",
			"tenantID": tenantTwo.ID,
			"brandID":  brandTwo.ID,
			"agentID":  agentTwo.ID,
			"amount":   9,
			"currency": "CNY",
			"reason":   "out of scope create",
			"freeze":   false,
		})
		require.Equal(t, http.StatusNotFound, createOutOfScope.Code, createOutOfScope.Body.String())
		assertJSONErrorBody(t, createOutOfScope, "record not found")

		var blockedCount int64
		require.NoError(t, testDB.Model(&model.RiskCase{}).Where("case_no = ?", "RISK-P3-CREATE-OUT").Count(&blockedCount).Error)
		require.Zero(t, blockedCount)
	})
}

func TestGameEndpointsAreBrandScoped(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenant := model.Tenant{Code: "GAME-BRAND-T1", Name: "Game Brand Tenant", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenant).Error)
	brandOne := model.Brand{TenantID: tenant.ID, Code: "GAME-BRAND-B1", Name: "Game Brand 1", Status: model.BrandStatusActive}
	brandTwo := model.Brand{TenantID: tenant.ID, Code: "GAME-BRAND-B2", Name: "Game Brand 2", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	require.NoError(t, testDB.Create(&brandTwo).Error)
	scopeTestUser(t, testDB, "admin", &tenant.ID, &brandOne.ID)

	agentOne := createAgentFixture(t, testDB, "AG-GB-1", "Game Brand Agent 1")
	agentTwo := createAgentFixture(t, testDB, "AG-GB-2", "Game Brand Agent 2")
	require.NoError(t, testDB.Model(&agentOne).Updates(map[string]any{"tenant_id": tenant.ID, "brand_id": brandOne.ID}).Error)
	require.NoError(t, testDB.Model(&agentTwo).Updates(map[string]any{"tenant_id": tenant.ID, "brand_id": brandTwo.ID}).Error)

	gameOne := createGameFixture(t, testDB, "GAME-GB-1", true)
	gameTwo := createGameFixture(t, testDB, "GAME-GB-2", true)
	require.NoError(t, testDB.Model(&gameOne).Updates(map[string]any{"tenant_id": tenant.ID, "brand_id": brandOne.ID}).Error)
	require.NoError(t, testDB.Model(&gameTwo).Updates(map[string]any{"tenant_id": tenant.ID, "brand_id": brandTwo.ID}).Error)

	keyOne := model.GameIntegrationKey{TenantID: &tenant.ID, BrandID: &brandOne.ID, GameID: gameOne.ID, Name: "gb-key-1", AccessKey: "ga_live_gb_1", SecretCiphertext: "gsk_live_gb_1", Status: model.GameIntegrationKeyStatusActive, Scopes: jsonStringArray(defaultGameOpenAPIScopes())}
	keyTwo := model.GameIntegrationKey{TenantID: &tenant.ID, BrandID: &brandTwo.ID, GameID: gameTwo.ID, Name: "gb-key-2", AccessKey: "ga_live_gb_2", SecretCiphertext: "gsk_live_gb_2", Status: model.GameIntegrationKeyStatusActive, Scopes: jsonStringArray(defaultGameOpenAPIScopes())}
	require.NoError(t, testDB.Create(&keyOne).Error)
	require.NoError(t, testDB.Create(&keyTwo).Error)

	accessOne := model.AgentGameAccess{TenantID: &tenant.ID, BrandID: &brandOne.ID, AgentID: agentOne.ID, GameID: gameOne.ID, Status: model.AccessStatusEnabled, GrantedBy: "seed", GrantedAt: time.Now().UTC(), EffectiveFrom: time.Now().UTC().Add(-time.Hour)}
	accessTwo := model.AgentGameAccess{TenantID: &tenant.ID, BrandID: &brandTwo.ID, AgentID: agentTwo.ID, GameID: gameTwo.ID, Status: model.AccessStatusEnabled, GrantedBy: "seed", GrantedAt: time.Now().UTC(), EffectiveFrom: time.Now().UTC().Add(-time.Hour)}
	require.NoError(t, testDB.Create(&accessOne).Error)
	require.NoError(t, testDB.Create(&accessTwo).Error)

	gamesResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/games", nil)
	require.Equal(t, http.StatusOK, gamesResp.Code, gamesResp.Body.String())
	var gameList struct {
		Items []model.Game `json:"items"`
		Total int          `json:"total"`
	}
	require.NoError(t, json.Unmarshal(gamesResp.Body.Bytes(), &gameList))
	require.Equal(t, 1, gameList.Total)
	require.Equal(t, gameOne.ID, gameList.Items[0].ID)

	accessResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agent-game-access", nil)
	require.Equal(t, http.StatusOK, accessResp.Code, accessResp.Body.String())
	var accessList struct {
		Items []agentGameAccessListItem `json:"items"`
		Total int                       `json:"total"`
	}
	require.NoError(t, json.Unmarshal(accessResp.Body.Bytes(), &accessList))
	require.Equal(t, 1, accessList.Total)
	require.Equal(t, accessOne.ID, accessList.Items[0].ID)

	keysResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/games/"+jsonUint(gameOne.ID)+"/integration-keys", nil)
	require.Equal(t, http.StatusOK, keysResp.Code, keysResp.Body.String())
	var keyList struct {
		Items []gameCredentialResponse `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(keysResp.Body.Bytes(), &keyList))
	require.Equal(t, 1, keyList.Total)
	require.Equal(t, keyOne.ID, keyList.Items[0].ID)

	outOfScopeKeysResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/games/"+jsonUint(gameTwo.ID)+"/integration-keys", nil)
	require.Equal(t, http.StatusNotFound, outOfScopeKeysResp.Code, outOfScopeKeysResp.Body.String())

	updateGameResp := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/games/"+jsonUint(gameTwo.ID), map[string]any{"name": "blocked"})
	require.Equal(t, http.StatusNotFound, updateGameResp.Code, updateGameResp.Body.String())
	assertJSONErrorBody(t, updateGameResp, "record not found")

	rotateKeyResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/games/"+jsonUint(gameTwo.ID)+"/integration-keys/"+jsonUint(keyTwo.ID)+"/rotate", map[string]any{})
	require.Equal(t, http.StatusNotFound, rotateKeyResp.Code, rotateKeyResp.Body.String())
	assertJSONErrorBody(t, rotateKeyResp, "record not found")

	revokeAccessResp := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/agent-game-access?agentID="+jsonUint(agentTwo.ID)+"&gameID="+jsonUint(gameTwo.ID), nil)
	require.Equal(t, http.StatusNotFound, revokeAccessResp.Code, revokeAccessResp.Body.String())
	assertJSONErrorBody(t, revokeAccessResp, "record not found")
}

func seedRBAC(t *testing.T, db *gorm.DB) map[string]string {
	t.Helper()

	permissions := []model.AdminPermission{
		{Code: "agent:read", Name: "Agent Read"},
		{Code: "agent:write", Name: "Agent Write"},
		{Code: "tenant:read", Name: "Tenant Read"},
		{Code: "tenant:write", Name: "Tenant Write"},
		{Code: "brand:read", Name: "Brand Read"},
		{Code: "brand:write", Name: "Brand Write"},
		{Code: "invite_code:manage", Name: "Invite Code Manage"},
		{Code: "player:read", Name: "Player Read"},
		{Code: "player:write", Name: "Player Write"},
		{Code: "binding:manage", Name: "Binding Manage"},
		{Code: "game:read", Name: "Game Read"},
		{Code: "game:write", Name: "Game Write"},
		{Code: "game:publish", Name: "Game Publish"},
		{Code: "agent_game_access:read", Name: "Agent Game Access Read"},
		{Code: "agent_game_access:write", Name: "Agent Game Access Write"},
		{Code: "rule:read", Name: "Rule Read"},
		{Code: "rule:write", Name: "Rule Write"},
		{Code: "rule:publish", Name: "Rule Publish"},
		{Code: "activity_reward:read", Name: "Activity Reward Read"},
		{Code: "activity_reward:write", Name: "Activity Reward Write"},
		{Code: "activity_reward:publish", Name: "Activity Reward Publish"},
		{Code: "settlement:read", Name: "Settlement Read"},
		{Code: "settlement:execute", Name: "Settlement Execute"},
		{Code: "withdrawal:read", Name: "Withdrawal Read"},
		{Code: "withdrawal:execute", Name: "Withdrawal Execute"},
		{Code: "settlement_bill:read", Name: "Settlement Bill Read"},
		{Code: "settlement_bill:confirm", Name: "Settlement Bill Confirm"},
		{Code: "settlement_bill:export", Name: "Settlement Bill Export"},
		{Code: "recalculation_task:read", Name: "Recalculation Task Read"},
		{Code: "recalculation_task:create", Name: "Recalculation Task Create"},
		{Code: "audit:read", Name: "Audit Read"},
		{Code: "risk:read", Name: "Risk Read"},
		{Code: "risk:write", Name: "Risk Write"},
		{Code: "platform_config:read", Name: "Platform Config Read"},
		{Code: "platform_config:write", Name: "Platform Config Write"},
		{Code: "report:read", Name: "Report Read"},
		{Code: "rbac:permissions:view", Name: "RBAC Permissions View"},
		{Code: "rbac:users:view", Name: "RBAC Users View"},
		{Code: "rbac:users:write", Name: "RBAC Users Write"},
		{Code: "rbac:roles:view", Name: "RBAC Roles View"},
		{Code: "rbac:roles:write", Name: "RBAC Roles Write"},
	}
	require.NoError(t, db.Create(&permissions).Error)

	permissionByCode := make(map[string]model.AdminPermission, len(permissions))
	for _, permission := range permissions {
		permissionByCode[permission.Code] = permission
	}

	roles := []model.AdminRole{{Code: "admin", Name: "Admin"}, {Code: "operator", Name: "Operator"}, {Code: "finance", Name: "Finance"}}
	require.NoError(t, db.Create(&roles).Error)

	roleByCode := make(map[string]model.AdminRole, len(roles))
	for _, role := range roles {
		roleByCode[role.Code] = role
	}

	for _, binding := range []struct {
		role        string
		permissions []string
	}{
		{role: "admin", permissions: []string{"agent:read", "agent:write", "tenant:read", "tenant:write", "brand:read", "brand:write", "invite_code:manage", "player:read", "player:write", "binding:manage", "game:read", "game:write", "game:publish", "agent_game_access:read", "agent_game_access:write", "rule:read", "rule:write", "rule:publish", "activity_reward:read", "activity_reward:write", "activity_reward:publish", "settlement:read", "settlement:execute", "withdrawal:read", "withdrawal:execute", "settlement_bill:read", "settlement_bill:confirm", "settlement_bill:export", "recalculation_task:read", "recalculation_task:create", "audit:read", "risk:read", "risk:write", "platform_config:read", "platform_config:write", "report:read", "rbac:permissions:view", "rbac:users:view", "rbac:users:write", "rbac:roles:view", "rbac:roles:write"}},
		{role: "operator", permissions: []string{"agent:read", "tenant:read", "brand:read", "player:read", "game:read", "agent_game_access:read", "rule:read", "activity_reward:read", "settlement:read", "rbac:permissions:view", "rbac:roles:view"}},
		{role: "finance", permissions: []string{"game:read", "rule:read", "settlement:read", "settlement:execute", "withdrawal:read", "withdrawal:execute", "settlement_bill:read", "settlement_bill:export", "recalculation_task:read", "recalculation_task:create", "risk:read", "risk:write", "platform_config:read", "report:read"}},
	} {
		for _, code := range binding.permissions {
			require.NoError(t, db.Create(&model.AdminRolePermission{RoleID: roleByCode[binding.role].ID, PermissionID: permissionByCode[code].ID}).Error)
		}
	}

	users := []struct {
		Username string
		Password string
		RoleCode string
	}{
		{Username: "admin", Password: "admin123", RoleCode: "admin"},
		{Username: "operator", Password: "operator123", RoleCode: "operator"},
		{Username: "finance", Password: "finance123", RoleCode: "finance"},
	}
	tokens := make(map[string]string, len(users))
	for _, user := range users {
		account := model.AdminUser{Username: user.Username, PasswordHash: user.Password, DisplayName: user.Username, Status: model.AdminUserStatusActive}
		require.NoError(t, db.Create(&account).Error)
		require.NoError(t, db.Create(&model.AdminUserRole{UserID: account.ID, RoleID: roleByCode[user.RoleCode].ID}).Error)
		tokens[user.Username] = user.Username
	}
	return tokens
}

func scopeTestUser(t *testing.T, db *gorm.DB, username string, tenantID *uint64, brandID *uint64) {
	t.Helper()
	var user model.AdminUser
	require.NoError(t, db.Where("username = ?", username).First(&user).Error)
	require.NoError(t, db.Model(&model.AdminUserRole{}).Where("user_id = ?", user.ID).Updates(map[string]any{"tenant_id": tenantID, "brand_id": brandID}).Error)
}

func findRbacUserIDByUsername(t *testing.T, items []rbacUserResponse, username string) uint64 {
	t.Helper()
	for _, item := range items {
		if item.Username == username {
			return item.ID
		}
	}
	require.Failf(t, "user not found", "username %s not found", username)
	return 0
}

func rbacUserListContainsUsername(items []rbacUserResponse, username string) bool {
	for _, item := range items {
		if item.Username == username {
			return true
		}
	}
	return false
}

func createScopedAdminRole(t *testing.T, db *gorm.DB, code string, tenantID *uint64, brandID *uint64, permissionCodes []string) model.AdminRole {
	t.Helper()
	role := model.AdminRole{TenantID: tenantID, BrandID: brandID, Code: code, Name: code}
	require.NoError(t, db.Create(&role).Error)
	var permissions []model.AdminPermission
	require.NoError(t, db.Where("code IN ?", permissionCodes).Find(&permissions).Error)
	require.Len(t, permissions, len(permissionCodes))
	for _, permission := range permissions {
		require.NoError(t, db.Create(&model.AdminRolePermission{RoleID: role.ID, PermissionID: permission.ID}).Error)
	}
	return role
}

func createScopedAdminUser(t *testing.T, db *gorm.DB, username string, password string, roleCode string, tenantID *uint64, brandID *uint64, agentID *uint64) model.AdminUser {
	t.Helper()
	var role model.AdminRole
	require.NoError(t, db.Where("code = ?", roleCode).First(&role).Error)
	user := model.AdminUser{Username: username, PasswordHash: password, DisplayName: username, Status: model.AdminUserStatusActive, AgentID: agentID}
	require.NoError(t, db.Create(&user).Error)
	require.NoError(t, db.Create(&model.AdminUserRole{UserID: user.ID, RoleID: role.ID, TenantID: tenantID, BrandID: brandID}).Error)
	return user
}

func loginAndAssertScope(t *testing.T, router http.Handler, username string, password string, level ScopeLevel, tenantID *uint64, brandID *uint64, agentID *uint64) authLoginResponse {
	t.Helper()
	resp := performJSONNoAuth(t, router, http.MethodPost, "/api/auth/login", map[string]any{"username": username, "password": password})
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body authLoginResponse
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.Equal(t, level, body.User.Scope.Level)
	assertOptionalUint64(t, tenantID, body.User.Scope.TenantID)
	assertOptionalUint64(t, brandID, body.User.Scope.BrandID)
	assertOptionalUint64(t, agentID, body.User.Scope.AgentID)
	require.NotEmpty(t, body.Token)
	return body
}

func assertOptionalUint64(t *testing.T, expected *uint64, actual *uint64) {
	t.Helper()
	if expected == nil {
		require.Nil(t, actual)
		return
	}
	require.NotNil(t, actual)
	require.Equal(t, *expected, *actual)
}

func assertAgentListIDs(t *testing.T, resp *httptest.ResponseRecorder, expected []uint64) {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		Items []model.Agent `json:"items"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.ElementsMatch(t, expected, collectAgentIDs(body.Items))
}

func assertPlayerListIDs(t *testing.T, resp *httptest.ResponseRecorder, expected []uint64) {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		Items []model.Player `json:"items"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.ElementsMatch(t, expected, collectPlayerIDs(body.Items))
}

func assertBindingHistoryPlayerIDs(t *testing.T, resp *httptest.ResponseRecorder, expected []uint64) {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		Items []model.BindingHistory `json:"items"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	actual := make([]uint64, 0, len(body.Items))
	for _, item := range body.Items {
		actual = append(actual, item.PlayerID)
	}
	require.ElementsMatch(t, expected, actual)
}

func assertPlatformConfigKeys(t *testing.T, resp *httptest.ResponseRecorder, expected []string) {
	t.Helper()
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	var body struct {
		Items []platformConfigListItem `json:"items"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	actual := make([]string, 0, len(body.Items))
	for _, item := range body.Items {
		actual = append(actual, item.Key)
	}
	require.ElementsMatch(t, expected, actual)
}

func seedBindingFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	agent := model.Agent{AgentNo: "AG-1", Name: "Seed Agent", DisplayName: "Seed Agent", Status: model.AgentStatusActive, Level: 1}
	player := model.Player{PlayerNo: "PL-1", PlatformUserID: "platform-user-1", Nickname: "P1", Currency: "CNY", Status: "active", RegisteredAt: time.Now().UTC()}
	require.NoError(t, db.Create(&agent).Error)
	require.NoError(t, db.Create(&player).Error)
}

func minimalRechargeCallbackPayload() map[string]any {
	return map[string]any{
		"orderNo":     "missing-order",
		"channelCode": "test",
		"status":      "success",
		"amount":      10,
		"paidAt":      time.Now().UTC().Format(time.RFC3339),
		"payload":     map[string]any{"source": "test"},
	}
}

func jsonBodyReader(t *testing.T, body any) io.Reader {
	t.Helper()
	if body == nil {
		return nil
	}
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return bytes.NewReader(payload)
}

func assertJSONErrorBody(t *testing.T, resp *httptest.ResponseRecorder, expectedSubstring string) {
	t.Helper()
	require.Equal(t, "application/json", resp.Header().Get("Content-Type"))
	var body struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		Error     string `json:"error"`
		RequestID string `json:"requestID"`
		TraceID   string `json:"traceID"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	require.NotEmpty(t, body.Code)
	require.NotEmpty(t, body.Message)
	require.Contains(t, body.Message, expectedSubstring)
	require.Contains(t, body.Error, expectedSubstring)
	require.NotEmpty(t, body.RequestID)
	require.NotEmpty(t, body.TraceID)
}

func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	resetHTTPTestDBOnce(t)
	cfg := config.Config{
		Env:      "test",
		HTTPPort: "18080",
		Database: config.DatabaseConfig{DSN: filepath.Join(t.TempDir(), "router.db")},
	}
	db := openSQLiteForTest(t, cfg.Database.DSN)
	require.NoError(t, autoMigrateForTest(db))
	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}, DB: db})
	return router, db
}

func newIsolatedTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	cfg := config.Config{
		Env:      "test",
		HTTPPort: "18080",
		Database: config.DatabaseConfig{DSN: filepath.Join(t.TempDir(), "isolated-router.db")},
	}
	db := openSQLiteForTest(t, cfg.Database.DSN)
	require.NoError(t, autoMigrateForTest(db))
	require.NoError(t, db.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&model.AgentRelationClosure{}).Error)
	router := NewRouter(RouterDependencies{Health: stubHealthProvider{}, DB: db})
	return router, db
}

var resetHTTPTestDB sync.Once

func resetHTTPTestDBOnce(t *testing.T) {
	t.Helper()
	resetHTTPTestDB.Do(func() {
		db := openSQLiteForTest(t, filepath.Join(t.TempDir(), "reset.db"))
		migrator := db.Migrator()
		for _, table := range []string{
			"brand",
			"tenant",
			"admin_role_permission",
			"admin_user_role",
			"admin_permission",
			"admin_role",
			"admin_user",
			"operation_audit_log",
			"recalculation_task",
			"settlement_bill_detail",
			"settlement_bill",
			"agent_account_ledger",
			"agent_account",
			"commission_record",
			"recharge_callback_log",
			"recharge_order",
			"rule_snapshot",
			"commission_rule",
			"agent_game_access",
			"game",
			"agent_invite_application",
			"agent_relation_closure",
			"agent_relation",
			"user_agent_binding_history",
			"user_agent_binding",
			"player",
			"agent_invite_code",
			"agent",
			"system_infos",
		} {
			_ = migrator.DropTable(table)
		}
		require.NoError(t, autoMigrateForTest(db))
	})
}

func openSQLiteForTest(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	require.NotEmpty(t, dsn)
	dir := filepath.Dir(dsn)
	if dir != "." && dir != "" {
		require.NoError(t, os.MkdirAll(dir, 0o755))
	}
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	return db
}

func autoMigrateForTest(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	models := append([]any{&systemInfoForTest{}}, model.Phase1Models...)
	return db.AutoMigrate(models...)
}

type systemInfoForTest struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"size:128;not null;uniqueIndex"`
	CreatedAt int64  `gorm:"autoCreateTime:milli"`
	UpdatedAt int64  `gorm:"autoUpdateTime:milli"`
}

func (systemInfoForTest) TableName() string {
	return "system_infos"
}

func performJSON(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return performRequestWithAuth(t, router, method, path, bytes.NewReader(payload))
}

func performJSONWithToken(t *testing.T, router http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return performRequestWithToken(t, router, token, method, path, bytes.NewReader(payload))
}

func performJSONNoAuth(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return performRequestNoAuth(t, router, method, path, bytes.NewReader(payload))
}

func performSignedGameJSON(t *testing.T, router http.Handler, method, path, accessKey, secret string, body any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	timestampValue := time.Now().UTC().Format(time.RFC3339)
	nonce := "nonce-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	bodyHash := sha256.Sum256(payload)
	signPayload := method + "\n" + path + "\n" + "" + "\n" + timestampValue + "\n" + nonce + "\n" + hex.EncodeToString(bodyHash[:])
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signPayload))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(gameAccessKeyHeader, accessKey)
	req.Header.Set(gameTimestampHeader, timestampValue)
	req.Header.Set(gameNonceHeader, nonce)
	req.Header.Set(gameSignatureHeader, signature)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func performMaybeJSONWithToken(t *testing.T, router http.Handler, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	if body == nil {
		return performRequestWithToken(t, router, token, method, path, nil)
	}
	return performJSONWithToken(t, router, token, method, path, body)
}

func performRequestWithAuth(t *testing.T, router http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	return performRequestWithToken(t, router, "admin", method, path, body)
}

func performRequestWithToken(t *testing.T, router http.Handler, token, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func performRequestNoAuth(t *testing.T, router http.Handler, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	return performRequestWithToken(t, router, "", method, path, body)
}

func mustDBFromRouter(t *testing.T, _ *gin.Engine) *gorm.DB {
	t.Helper()
	_, db := newTestRouter(t)
	return db
}

func jsonUint(v uint64) string {
	return strconv.FormatUint(v, 10)
}

func TestAgentHierarchyClosureQueriesAndTeamStats(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	root := createAgentFixture(t, testDB, "AG-ROOT", "Root Agent")
	childA := createAgentFixture(t, testDB, "AG-A", "Agent A")
	childB := createAgentFixture(t, testDB, "AG-B", "Agent B")
	grandchild := createAgentFixture(t, testDB, "AG-C", "Agent C")
	createInviteCodeFixture(t, testDB, root.ID, "INV-ROOT")
	createInviteCodeFixture(t, testDB, childA.ID, "INV-A")

	appA := applyAndApproveAgentInvite(t, router, childA.ID, "INV-ROOT")
	appB := applyAndApproveAgentInvite(t, router, childB.ID, "INV-ROOT")
	appC := applyAndApproveAgentInvite(t, router, grandchild.ID, "INV-A")
	require.NotNil(t, appA.ApprovedRelationID)
	require.NotNil(t, appB.ApprovedRelationID)
	require.NotNil(t, appC.ApprovedRelationID)

	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount: 3,
		closureCount:        8,
		depthCounts:         map[uint32]int{0: 4, 1: 3, 2: 1},
		selfClosures:        []uint64{root.ID, childA.ID, childB.ID, grandchild.ID},
		directClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: childA.ID, viaDirectParentID: uint64Ptr(root.ID), relationType: model.RelationTypeDirect},
			{ancestorID: root.ID, descendantID: childB.ID, viaDirectParentID: uint64Ptr(root.ID), relationType: model.RelationTypeDirect},
			{ancestorID: childA.ID, descendantID: grandchild.ID, viaDirectParentID: uint64Ptr(childA.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: grandchild.ID, depth: 2, viaDirectParentID: uint64Ptr(childA.ID), relationType: model.RelationTypeClosure},
		},
	})

	var refreshedGrandchild model.Agent
	require.NoError(t, testDB.First(&refreshedGrandchild, grandchild.ID).Error)
	require.NotNil(t, refreshedGrandchild.ParentAgentID)
	require.Equal(t, childA.ID, *refreshedGrandchild.ParentAgentID)

	ancestorsResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agents/"+jsonUint(grandchild.ID)+"/ancestors?maxDepth=1", nil)
	require.Equal(t, http.StatusOK, ancestorsResp.Code)
	var ancestorsBody struct {
		Items []struct {
			AncestorAgentID   uint64             `json:"ancestorAgentID"`
			DescendantAgentID uint64             `json:"descendantAgentID"`
			Depth             uint32             `json:"depth"`
			RelationType      model.RelationType `json:"relationType"`
			ViaDirectParentID *uint64            `json:"viaDirectParentID"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(ancestorsResp.Body.Bytes(), &ancestorsBody))
	require.Equal(t, 1, ancestorsBody.Total)
	require.Equal(t, childA.ID, ancestorsBody.Items[0].AncestorAgentID)
	require.Equal(t, grandchild.ID, ancestorsBody.Items[0].DescendantAgentID)
	require.EqualValues(t, 1, ancestorsBody.Items[0].Depth)
	require.Equal(t, model.RelationTypeDirect, ancestorsBody.Items[0].RelationType)
	require.NotNil(t, ancestorsBody.Items[0].ViaDirectParentID)
	require.Equal(t, childA.ID, *ancestorsBody.Items[0].ViaDirectParentID)

	descendantsResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agents/"+jsonUint(root.ID)+"/descendants?minDepth=2&maxDepth=2", nil)
	require.Equal(t, http.StatusOK, descendantsResp.Code)
	var descendantsBody struct {
		Items []struct {
			AncestorAgentID   uint64             `json:"ancestorAgentID"`
			DescendantAgentID uint64             `json:"descendantAgentID"`
			Depth             uint32             `json:"depth"`
			RelationType      model.RelationType `json:"relationType"`
			ViaDirectParentID *uint64            `json:"viaDirectParentID"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(descendantsResp.Body.Bytes(), &descendantsBody))
	require.Equal(t, 1, descendantsBody.Total)
	require.Equal(t, root.ID, descendantsBody.Items[0].AncestorAgentID)
	require.Equal(t, grandchild.ID, descendantsBody.Items[0].DescendantAgentID)
	require.EqualValues(t, 2, descendantsBody.Items[0].Depth)
	require.Equal(t, model.RelationTypeClosure, descendantsBody.Items[0].RelationType)
	require.NotNil(t, descendantsBody.Items[0].ViaDirectParentID)
	require.Equal(t, childA.ID, *descendantsBody.Items[0].ViaDirectParentID)

	statsResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agents/"+jsonUint(root.ID)+"/team-stats?minDepth=1&maxDepth=2", nil)
	require.Equal(t, http.StatusOK, statsResp.Code)
	var statsBody struct {
		AgentID            uint64 `json:"agentID"`
		DirectDescendants  int64  `json:"directDescendants"`
		TotalDescendants   int64  `json:"totalDescendants"`
		TotalAncestors     int64  `json:"totalAncestors"`
		MaxDescendantDepth uint32 `json:"maxDescendantDepth"`
		MaxAncestorDepth   uint32 `json:"maxAncestorDepth"`
		LeafDescendants    int64  `json:"leafDescendants"`
		DepthBreakdown     []struct {
			Depth uint32 `json:"depth"`
			Count int64  `json:"count"`
		} `json:"depthBreakdown"`
	}
	require.NoError(t, json.Unmarshal(statsResp.Body.Bytes(), &statsBody))
	require.Equal(t, root.ID, statsBody.AgentID)
	require.EqualValues(t, 2, statsBody.DirectDescendants)
	require.EqualValues(t, 3, statsBody.TotalDescendants)
	require.EqualValues(t, 0, statsBody.TotalAncestors)
	require.EqualValues(t, 2, statsBody.MaxDescendantDepth)
	require.EqualValues(t, 2, statsBody.LeafDescendants)
	require.Len(t, statsBody.DepthBreakdown, 2)
	require.EqualValues(t, 1, statsBody.DepthBreakdown[0].Depth)
	require.EqualValues(t, 2, statsBody.DepthBreakdown[0].Count)
	require.EqualValues(t, 2, statsBody.DepthBreakdown[1].Depth)
	require.EqualValues(t, 1, statsBody.DepthBreakdown[1].Count)
}

func TestAgentHierarchyQueriesSupportMultiDepthMetadataAndFilteredStats(t *testing.T) {
	t.Parallel()

	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	root := createAgentFixture(t, testDB, "AG-HIER-ROOT", "Hierarchy Root")
	childA := createAgentFixture(t, testDB, "AG-HIER-A", "Hierarchy A")
	childB := createAgentFixture(t, testDB, "AG-HIER-B", "Hierarchy B")
	grandchildA := createAgentFixture(t, testDB, "AG-HIER-A1", "Hierarchy A1")
	grandchildB := createAgentFixture(t, testDB, "AG-HIER-A2", "Hierarchy A2")
	greatGrandchild := createAgentFixture(t, testDB, "AG-HIER-A3", "Hierarchy A3")

	require.NoError(t, createDirectAgentRelation(t, testDB, root.ID, childA.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, root.ID, childB.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, childA.ID, grandchildA.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, childA.ID, grandchildB.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, grandchildA.ID, greatGrandchild.ID))

	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount: 5,
		closureCount:        15,
		depthCounts:         map[uint32]int{0: 6, 1: 5, 2: 3, 3: 1},
		selfClosures:        []uint64{root.ID, childA.ID, childB.ID, grandchildA.ID, grandchildB.ID, greatGrandchild.ID},
		directClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: childA.ID, viaDirectParentID: uint64Ptr(root.ID), relationType: model.RelationTypeDirect},
			{ancestorID: root.ID, descendantID: childB.ID, viaDirectParentID: uint64Ptr(root.ID), relationType: model.RelationTypeDirect},
			{ancestorID: childA.ID, descendantID: grandchildA.ID, viaDirectParentID: uint64Ptr(childA.ID), relationType: model.RelationTypeDirect},
			{ancestorID: childA.ID, descendantID: grandchildB.ID, viaDirectParentID: uint64Ptr(childA.ID), relationType: model.RelationTypeDirect},
			{ancestorID: grandchildA.ID, descendantID: greatGrandchild.ID, viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: grandchildA.ID, depth: 2, viaDirectParentID: uint64Ptr(childA.ID), relationType: model.RelationTypeClosure},
			{ancestorID: root.ID, descendantID: grandchildB.ID, depth: 2, viaDirectParentID: uint64Ptr(childA.ID), relationType: model.RelationTypeClosure},
			{ancestorID: childA.ID, descendantID: greatGrandchild.ID, depth: 2, viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeClosure},
			{ancestorID: root.ID, descendantID: greatGrandchild.ID, depth: 3, viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeClosure},
		},
	})

	ancestorsResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agents/"+jsonUint(greatGrandchild.ID)+"/ancestors?minDepth=1&maxDepth=3", nil)
	require.Equal(t, http.StatusOK, ancestorsResp.Code)
	var ancestorsBody struct {
		Items []struct {
			AncestorAgentID   uint64             `json:"ancestorAgentID"`
			DescendantAgentID uint64             `json:"descendantAgentID"`
			Depth             uint32             `json:"depth"`
			RelationType      model.RelationType `json:"relationType"`
			ViaDirectParentID *uint64            `json:"viaDirectParentID"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(ancestorsResp.Body.Bytes(), &ancestorsBody))
	require.Equal(t, 3, ancestorsBody.Total)
	require.Equal(t, grandchildA.ID, ancestorsBody.Items[0].AncestorAgentID)
	require.EqualValues(t, 1, ancestorsBody.Items[0].Depth)
	require.Equal(t, model.RelationTypeDirect, ancestorsBody.Items[0].RelationType)
	require.NotNil(t, ancestorsBody.Items[0].ViaDirectParentID)
	require.Equal(t, grandchildA.ID, *ancestorsBody.Items[0].ViaDirectParentID)
	require.Equal(t, childA.ID, ancestorsBody.Items[1].AncestorAgentID)
	require.EqualValues(t, 2, ancestorsBody.Items[1].Depth)
	require.Equal(t, model.RelationTypeClosure, ancestorsBody.Items[1].RelationType)
	require.NotNil(t, ancestorsBody.Items[1].ViaDirectParentID)
	require.Equal(t, grandchildA.ID, *ancestorsBody.Items[1].ViaDirectParentID)
	require.Equal(t, root.ID, ancestorsBody.Items[2].AncestorAgentID)
	require.EqualValues(t, 3, ancestorsBody.Items[2].Depth)
	require.Equal(t, model.RelationTypeClosure, ancestorsBody.Items[2].RelationType)
	require.NotNil(t, ancestorsBody.Items[2].ViaDirectParentID)
	require.Equal(t, grandchildA.ID, *ancestorsBody.Items[2].ViaDirectParentID)

	descendantsResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agents/"+jsonUint(root.ID)+"/descendants?minDepth=1&maxDepth=3", nil)
	require.Equal(t, http.StatusOK, descendantsResp.Code)
	var descendantsBody struct {
		Items []struct {
			AncestorAgentID   uint64             `json:"ancestorAgentID"`
			DescendantAgentID uint64             `json:"descendantAgentID"`
			Depth             uint32             `json:"depth"`
			RelationType      model.RelationType `json:"relationType"`
			ViaDirectParentID *uint64            `json:"viaDirectParentID"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(descendantsResp.Body.Bytes(), &descendantsBody))
	require.Equal(t, 5, descendantsBody.Total)
	require.Equal(t, childA.ID, descendantsBody.Items[0].DescendantAgentID)
	require.EqualValues(t, 1, descendantsBody.Items[0].Depth)
	require.Equal(t, model.RelationTypeDirect, descendantsBody.Items[0].RelationType)
	require.NotNil(t, descendantsBody.Items[0].ViaDirectParentID)
	require.Equal(t, root.ID, *descendantsBody.Items[0].ViaDirectParentID)
	require.Equal(t, childB.ID, descendantsBody.Items[1].DescendantAgentID)
	require.EqualValues(t, 1, descendantsBody.Items[1].Depth)
	require.Equal(t, grandchildA.ID, descendantsBody.Items[2].DescendantAgentID)
	require.EqualValues(t, 2, descendantsBody.Items[2].Depth)
	require.Equal(t, model.RelationTypeClosure, descendantsBody.Items[2].RelationType)
	require.NotNil(t, descendantsBody.Items[2].ViaDirectParentID)
	require.Equal(t, childA.ID, *descendantsBody.Items[2].ViaDirectParentID)
	require.Equal(t, grandchildB.ID, descendantsBody.Items[3].DescendantAgentID)
	require.EqualValues(t, 2, descendantsBody.Items[3].Depth)
	require.Equal(t, greatGrandchild.ID, descendantsBody.Items[4].DescendantAgentID)
	require.EqualValues(t, 3, descendantsBody.Items[4].Depth)
	require.Equal(t, model.RelationTypeClosure, descendantsBody.Items[4].RelationType)
	require.NotNil(t, descendantsBody.Items[4].ViaDirectParentID)
	require.Equal(t, grandchildA.ID, *descendantsBody.Items[4].ViaDirectParentID)

	statsResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/agents/"+jsonUint(childA.ID)+"/team-stats?minDepth=2&maxDepth=2", nil)
	require.Equal(t, http.StatusOK, statsResp.Code)
	var statsBody struct {
		AgentID            uint64 `json:"agentID"`
		DirectDescendants  int64  `json:"directDescendants"`
		TotalDescendants   int64  `json:"totalDescendants"`
		TotalAncestors     int64  `json:"totalAncestors"`
		MaxDescendantDepth uint32 `json:"maxDescendantDepth"`
		MaxAncestorDepth   uint32 `json:"maxAncestorDepth"`
		LeafDescendants    int64  `json:"leafDescendants"`
		DepthBreakdown     []struct {
			Depth uint32 `json:"depth"`
			Count int64  `json:"count"`
		} `json:"depthBreakdown"`
	}
	require.NoError(t, json.Unmarshal(statsResp.Body.Bytes(), &statsBody))
	require.Equal(t, childA.ID, statsBody.AgentID)
	require.EqualValues(t, 2, statsBody.DirectDescendants)
	require.EqualValues(t, 1, statsBody.TotalDescendants)
	require.EqualValues(t, 0, statsBody.TotalAncestors)
	require.EqualValues(t, 2, statsBody.MaxDescendantDepth)
	require.EqualValues(t, 0, statsBody.MaxAncestorDepth)
	require.EqualValues(t, 1, statsBody.LeafDescendants)
	require.Len(t, statsBody.DepthBreakdown, 1)
	require.EqualValues(t, 2, statsBody.DepthBreakdown[0].Depth)
	require.EqualValues(t, 1, statsBody.DepthBreakdown[0].Count)
}

func TestCreateAgentRelationRebuildsClosuresAfterParentChanges(t *testing.T) {
	t.Parallel()

	_, testDB := newIsolatedTestRouter(t)

	rootA := createAgentFixture(t, testDB, "AG-ROOT-A", "Root A")
	rootB := createAgentFixture(t, testDB, "AG-ROOT-B", "Root B")
	child := createAgentFixture(t, testDB, "AG-CHILD", "Child")
	grandchild := createAgentFixture(t, testDB, "AG-GRANDCHILD", "Grandchild")

	require.NoError(t, createDirectAgentRelation(t, testDB, rootA.ID, child.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, child.ID, grandchild.ID))
	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount: 2,
		closureCount:        7,
		depthCounts:         map[uint32]int{0: 4, 1: 2, 2: 1},
		selfClosures:        []uint64{rootA.ID, rootB.ID, child.ID, grandchild.ID},
		directClosures: []closureExpectation{
			{ancestorID: rootA.ID, descendantID: child.ID, viaDirectParentID: uint64Ptr(rootA.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchild.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: rootA.ID, descendantID: grandchild.ID, depth: 2, pathSnapshot: jsonUint(rootA.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchild.ID), viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
		},
	})

	require.NoError(t, testDB.Model(&model.Agent{}).Where("id = ?", child.ID).Update("parent_agent_id", nil).Error)
	require.NoError(t, testDB.Session(&gorm.Session{}).Transaction(func(tx *gorm.DB) error {
		_, err := createAgentRelation(tx, rootB.ID, child.ID)
		return err
	}))

	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount:   2,
		inactiveRelationCount: 1,
		closureCount:          7,
		depthCounts:           map[uint32]int{0: 4, 1: 2, 2: 1},
		selfClosures:          []uint64{rootA.ID, rootB.ID, child.ID, grandchild.ID},
		directClosures: []closureExpectation{
			{ancestorID: rootB.ID, descendantID: child.ID, viaDirectParentID: uint64Ptr(rootB.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchild.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: rootB.ID, descendantID: grandchild.ID, depth: 2, pathSnapshot: jsonUint(rootB.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchild.ID), viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
		},
		missingClosures: []closureKey{
			{ancestorID: rootA.ID, descendantID: child.ID, depth: 1},
			{ancestorID: rootA.ID, descendantID: grandchild.ID, depth: 2},
		},
	})

	var refreshedChild model.Agent
	require.NoError(t, testDB.First(&refreshedChild, child.ID).Error)
	require.NotNil(t, refreshedChild.ParentAgentID)
	require.Equal(t, rootB.ID, *refreshedChild.ParentAgentID)

	var childRelations []model.Relation
	require.NoError(t, testDB.Where("descendant_agent_id = ? AND depth = ?", child.ID, 1).Order("id asc").Find(&childRelations).Error)
	require.Len(t, childRelations, 2)
	require.Equal(t, rootA.ID, childRelations[0].AncestorAgentID)
	require.Equal(t, model.RelationStatusInactive, childRelations[0].Status)
	require.NotNil(t, childRelations[0].EffectiveTo)
	require.Equal(t, rootB.ID, childRelations[1].AncestorAgentID)
	require.Equal(t, model.RelationStatusActive, childRelations[1].Status)
	require.Nil(t, childRelations[1].EffectiveTo)
}

func TestCreateAgentRelationPreservesSiblingClosuresWhenReparentingBranch(t *testing.T) {
	t.Parallel()

	_, testDB := newIsolatedTestRouter(t)

	rootA := createAgentFixture(t, testDB, "AG-BRANCH-ROOT-A", "Branch Root A")
	rootB := createAgentFixture(t, testDB, "AG-BRANCH-ROOT-B", "Branch Root B")
	child := createAgentFixture(t, testDB, "AG-BRANCH-CHILD", "Branch Child")
	grandchildA := createAgentFixture(t, testDB, "AG-BRANCH-GRAND-A", "Branch Grandchild A")
	grandchildB := createAgentFixture(t, testDB, "AG-BRANCH-GRAND-B", "Branch Grandchild B")
	greatGrandchild := createAgentFixture(t, testDB, "AG-BRANCH-GREAT", "Branch Great Grandchild")
	sibling := createAgentFixture(t, testDB, "AG-BRANCH-SIBLING", "Branch Sibling")

	require.NoError(t, createDirectAgentRelation(t, testDB, rootA.ID, child.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, child.ID, grandchildA.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, child.ID, grandchildB.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, grandchildA.ID, greatGrandchild.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, rootA.ID, sibling.ID))

	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount: 5,
		closureCount:        16,
		depthCounts:         map[uint32]int{0: 7, 1: 5, 2: 3, 3: 1},
		selfClosures:        []uint64{rootA.ID, rootB.ID, child.ID, grandchildA.ID, grandchildB.ID, greatGrandchild.ID, sibling.ID},
		directClosures: []closureExpectation{
			{ancestorID: rootA.ID, descendantID: child.ID, viaDirectParentID: uint64Ptr(rootA.ID), relationType: model.RelationTypeDirect},
			{ancestorID: rootA.ID, descendantID: sibling.ID, viaDirectParentID: uint64Ptr(rootA.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchildA.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchildB.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
			{ancestorID: grandchildA.ID, descendantID: greatGrandchild.ID, viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: rootA.ID, descendantID: grandchildA.ID, depth: 2, pathSnapshot: jsonUint(rootA.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchildA.ID), viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
			{ancestorID: rootA.ID, descendantID: grandchildB.ID, depth: 2, pathSnapshot: jsonUint(rootA.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchildB.ID), viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
			{ancestorID: child.ID, descendantID: greatGrandchild.ID, depth: 2, pathSnapshot: jsonUint(child.ID) + "/" + jsonUint(grandchildA.ID) + "/" + jsonUint(greatGrandchild.ID), viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeClosure},
			{ancestorID: rootA.ID, descendantID: greatGrandchild.ID, depth: 3, pathSnapshot: jsonUint(rootA.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchildA.ID) + "/" + jsonUint(greatGrandchild.ID), viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeClosure},
		},
	})

	require.NoError(t, testDB.Model(&model.Agent{}).Where("id = ?", child.ID).Update("parent_agent_id", nil).Error)
	require.NoError(t, testDB.Transaction(func(tx *gorm.DB) error {
		_, err := createAgentRelation(tx, rootB.ID, child.ID)
		return err
	}))

	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount:   5,
		inactiveRelationCount: 1,
		closureCount:          16,
		depthCounts:           map[uint32]int{0: 7, 1: 5, 2: 3, 3: 1},
		selfClosures:          []uint64{rootA.ID, rootB.ID, child.ID, grandchildA.ID, grandchildB.ID, greatGrandchild.ID, sibling.ID},
		directClosures: []closureExpectation{
			{ancestorID: rootB.ID, descendantID: child.ID, viaDirectParentID: uint64Ptr(rootB.ID), relationType: model.RelationTypeDirect},
			{ancestorID: rootA.ID, descendantID: sibling.ID, viaDirectParentID: uint64Ptr(rootA.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchildA.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchildB.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
			{ancestorID: grandchildA.ID, descendantID: greatGrandchild.ID, viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: rootB.ID, descendantID: grandchildA.ID, depth: 2, pathSnapshot: jsonUint(rootB.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchildA.ID), viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
			{ancestorID: rootB.ID, descendantID: grandchildB.ID, depth: 2, pathSnapshot: jsonUint(rootB.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchildB.ID), viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
			{ancestorID: child.ID, descendantID: greatGrandchild.ID, depth: 2, pathSnapshot: jsonUint(child.ID) + "/" + jsonUint(grandchildA.ID) + "/" + jsonUint(greatGrandchild.ID), viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeClosure},
			{ancestorID: rootB.ID, descendantID: greatGrandchild.ID, depth: 3, pathSnapshot: jsonUint(rootB.ID) + "/" + jsonUint(child.ID) + "/" + jsonUint(grandchildA.ID) + "/" + jsonUint(greatGrandchild.ID), viaDirectParentID: uint64Ptr(grandchildA.ID), relationType: model.RelationTypeClosure},
		},
		missingClosures: []closureKey{
			{ancestorID: rootA.ID, descendantID: child.ID, depth: 1},
			{ancestorID: rootA.ID, descendantID: grandchildA.ID, depth: 2},
			{ancestorID: rootA.ID, descendantID: grandchildB.ID, depth: 2},
			{ancestorID: rootA.ID, descendantID: greatGrandchild.ID, depth: 3},
			{ancestorID: rootB.ID, descendantID: sibling.ID, depth: 1},
		},
	})
}

func TestCreateAgentRelationDetectsCyclesUsingClosureTable(t *testing.T) {
	t.Parallel()

	_, testDB := newIsolatedTestRouter(t)

	root := createAgentFixture(t, testDB, "AG-CYCLE-ROOT", "Cycle Root")
	child := createAgentFixture(t, testDB, "AG-CYCLE-CHILD", "Cycle Child")
	grandchild := createAgentFixture(t, testDB, "AG-CYCLE-GRAND", "Cycle Grand")

	require.NoError(t, createDirectAgentRelation(t, testDB, root.ID, child.ID))
	require.NoError(t, createDirectAgentRelation(t, testDB, child.ID, grandchild.ID))

	err := testDB.Transaction(func(tx *gorm.DB) error {
		_, err := createAgentRelation(tx, grandchild.ID, root.ID)
		return err
	})
	require.ErrorIs(t, err, errAgentInviteLoopDetected)

	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount: 2,
		closureCount:        6,
		depthCounts:         map[uint32]int{0: 3, 1: 2, 2: 1},
		selfClosures:        []uint64{root.ID, child.ID, grandchild.ID},
		directClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: child.ID, viaDirectParentID: uint64Ptr(root.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchild.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: grandchild.ID, depth: 2, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
		},
	})
}

func TestRebuildAgentRelationClosureRepairsStaleClosureRowsAndPreservesCurrentPaths(t *testing.T) {
	t.Parallel()

	_, testDB := newIsolatedTestRouter(t)

	root := createAgentFixture(t, testDB, "AG-REBUILD-ROOT", "Rebuild Root")
	child := createAgentFixture(t, testDB, "AG-REBUILD-CHILD", "Rebuild Child")
	grandchild := createAgentFixture(t, testDB, "AG-REBUILD-GRAND", "Rebuild Grandchild")
	orphan := createAgentFixture(t, testDB, "AG-REBUILD-ORPHAN", "Rebuild Orphan")

	require.NoError(t, testDB.Model(&model.Agent{}).Where("id = ?", child.ID).Update("parent_agent_id", root.ID).Error)
	require.NoError(t, testDB.Model(&model.Agent{}).Where("id = ?", grandchild.ID).Update("parent_agent_id", child.ID).Error)
	require.NoError(t, testDB.Model(&model.Agent{}).Where("id = ?", orphan.ID).Update("parent_agent_id", root.ID).Error)

	staleVia := orphan.ID
	require.NoError(t, testDB.Create(&model.AgentRelationClosure{
		AncestorAgentID:   root.ID,
		DescendantAgentID: grandchild.ID,
		Depth:             99,
		ViaDirectParentID: &staleVia,
		RelationType:      model.RelationTypeClosure,
		Status:            model.RelationStatusActive,
		EffectiveFrom:     time.Now().UTC(),
	}).Error)
	require.NoError(t, testDB.Create(&model.AgentRelationClosure{
		AncestorAgentID:   orphan.ID,
		DescendantAgentID: grandchild.ID,
		Depth:             1,
		ViaDirectParentID: &staleVia,
		RelationType:      model.RelationTypeDirect,
		Status:            model.RelationStatusActive,
		EffectiveFrom:     time.Now().UTC(),
	}).Error)

	require.NoError(t, testDB.Transaction(func(tx *gorm.DB) error {
		return rebuildAgentRelationClosure(tx)
	}))

	assertAgentHierarchyState(t, testDB, agentHierarchyExpectation{
		directRelationCount: 0,
		closureCount:        8,
		depthCounts:         map[uint32]int{0: 4, 1: 3, 2: 1},
		selfClosures:        []uint64{root.ID, child.ID, grandchild.ID, orphan.ID},
		directClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: child.ID, viaDirectParentID: uint64Ptr(root.ID), relationType: model.RelationTypeDirect},
			{ancestorID: child.ID, descendantID: grandchild.ID, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeDirect},
			{ancestorID: root.ID, descendantID: orphan.ID, viaDirectParentID: uint64Ptr(root.ID), relationType: model.RelationTypeDirect},
		},
		indirectClosures: []closureExpectation{
			{ancestorID: root.ID, descendantID: grandchild.ID, depth: 2, viaDirectParentID: uint64Ptr(child.ID), relationType: model.RelationTypeClosure},
		},
		missingClosures: []closureKey{
			{ancestorID: root.ID, descendantID: grandchild.ID, depth: 99},
			{ancestorID: orphan.ID, descendantID: grandchild.ID, depth: 1},
		},
	})
}

func TestAgentHierarchyEndpointsRejectInvalidAndMissingAgents(t *testing.T) {
	t.Parallel()

	router, testDB := newTestRouter(t)
	seedRBAC(t, testDB)
	_ = createAgentFixture(t, testDB, "AG-ONE", "Only Agent")

	for _, path := range []string{
		"/api/agents/not-a-number/ancestors",
		"/api/agents/not-a-number/descendants",
		"/api/agents/not-a-number/team-stats",
		"/api/agents/1/ancestors?minDepth=abc",
		"/api/agents/1/descendants?maxDepth=0",
		"/api/agents/1/team-stats?minDepth=3&maxDepth=2",
	} {
		resp := performRequestWithToken(t, router, "admin", http.MethodGet, path, nil)
		require.Equal(t, http.StatusBadRequest, resp.Code)
	}

	for _, path := range []string{
		"/api/agents/999999/ancestors",
		"/api/agents/999999/descendants",
		"/api/agents/999999/team-stats",
	} {
		resp := performRequestWithToken(t, router, "admin", http.MethodGet, path, nil)
		require.Equal(t, http.StatusNotFound, resp.Code)
	}

	for _, path := range []string{
		"/api/agents/1/ancestors",
		"/api/agents/1/descendants",
		"/api/agents/1/team-stats",
	} {
		resp := performRequestWithToken(t, router, "finance", http.MethodGet, path, nil)
		require.Equal(t, http.StatusForbidden, resp.Code)
	}
}

func createAgentFixture(t *testing.T, db *gorm.DB, agentNo, name string) model.Agent {
	t.Helper()
	agent := model.Agent{AgentNo: agentNo, Name: name, DisplayName: name, Status: model.AgentStatusActive, Level: 1, Remark: "t1"}
	require.NoError(t, db.Create(&agent).Error)
	return agent
}

func accountIDForAgent(t *testing.T, db *gorm.DB, agentID uint64) uint64 {
	t.Helper()
	var account model.AgentAccount
	require.NoError(t, db.Where("agent_id = ?", agentID).First(&account).Error)
	return account.ID
}

func createAgentAccountFixture(t *testing.T, db *gorm.DB, agentID uint64, accountNo string) model.AgentAccount {
	t.Helper()
	account := model.AgentAccount{
		AgentID:            agentID,
		AccountNo:          accountNo,
		Status:             model.AccountStatusActive,
		Currency:           "CNY",
		Balance:            0,
		AvailableBalance:   0,
		FrozenBalance:      0,
		WithdrawableAmount: 0,
		TotalIncome:        0,
		TotalReversed:      0,
		Version:            1,
	}
	require.NoError(t, db.Create(&account).Error)
	return account
}

func createInviteCodeFixture(t *testing.T, db *gorm.DB, agentID uint64, code string) model.InviteCode {
	t.Helper()
	invite := model.InviteCode{AgentID: agentID, Code: code, Status: model.InviteCodeStatusActive}
	require.NoError(t, db.Create(&invite).Error)
	return invite
}

func applyAndApproveAgentInvite(t *testing.T, router http.Handler, applicantAgentID uint64, inviteCode string) model.AgentInviteApplication {
	t.Helper()
	applyResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agent-invite-applications", map[string]any{
		"applicantAgentID": applicantAgentID,
		"inviteCode":       inviteCode,
	})
	require.Equal(t, http.StatusCreated, applyResp.Code, applyResp.Body.String())
	var application model.AgentInviteApplication
	require.NoError(t, json.Unmarshal(applyResp.Body.Bytes(), &application))

	auditResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agent-invite-applications/"+jsonUint(application.ID)+"/audit", map[string]any{
		"status": "approved",
	})
	if auditResp.Code != http.StatusOK {
		t.Fatalf("unexpected audit response: %s", auditResp.Body.String())
	}
	require.Equal(t, http.StatusOK, auditResp.Code, auditResp.Body.String())
	require.NoError(t, json.Unmarshal(auditResp.Body.Bytes(), &application))
	return application
}

func findClosure(t *testing.T, closures []model.AgentRelationClosure, ancestorID, descendantID uint64, depth uint32) model.AgentRelationClosure {
	t.Helper()
	for _, closure := range closures {
		if closure.AncestorAgentID == ancestorID && closure.DescendantAgentID == descendantID && closure.Depth == depth {
			return closure
		}
	}
	t.Fatalf("closure not found ancestor=%d descendant=%d depth=%d", ancestorID, descendantID, depth)
	return model.AgentRelationClosure{}
}

func createDirectAgentRelation(t *testing.T, db *gorm.DB, inviterAgentID, applicantAgentID uint64) error {
	t.Helper()
	return db.Transaction(func(tx *gorm.DB) error {
		_, err := createAgentRelation(tx, inviterAgentID, applicantAgentID)
		return err
	})
}

type closureKey struct {
	ancestorID   uint64
	descendantID uint64
	depth        uint32
}

type closureExpectation struct {
	ancestorID        uint64
	descendantID      uint64
	depth             uint32
	pathSnapshot      string
	viaDirectParentID *uint64
	relationType      model.RelationType
}

type agentHierarchyExpectation struct {
	directRelationCount   int
	inactiveRelationCount int
	closureCount          int
	selfClosures          []uint64
	directClosures        []closureExpectation
	indirectClosures      []closureExpectation
	missingClosures       []closureKey
	depthCounts           map[uint32]int
}

func assertAgentHierarchyState(t *testing.T, db *gorm.DB, expected agentHierarchyExpectation) {
	t.Helper()

	var directRelations []model.Relation
	require.NoError(t, db.Order("ancestor_agent_id asc, descendant_agent_id asc, depth asc, id asc").Find(&directRelations).Error)

	activeDirectCount := 0
	inactiveCount := 0
	for _, relation := range directRelations {
		require.Equal(t, model.RelationTypeDirect, relation.RelationType)
		require.EqualValues(t, 1, relation.Depth)
		require.NotNil(t, relation.DirectParentID)
		require.Equal(t, relation.AncestorAgentID, *relation.DirectParentID)
		switch relation.Status {
		case model.RelationStatusActive:
			activeDirectCount++
			require.Nil(t, relation.EffectiveTo)
		case model.RelationStatusInactive:
			inactiveCount++
			require.NotNil(t, relation.EffectiveTo)
		default:
			t.Fatalf("unexpected relation status %q", relation.Status)
		}
	}
	require.Equal(t, expected.directRelationCount, activeDirectCount)
	require.Equal(t, expected.inactiveRelationCount, inactiveCount)

	var closures []model.AgentRelationClosure
	require.NoError(t, db.Order("ancestor_agent_id asc, descendant_agent_id asc, depth asc").Find(&closures).Error)
	require.Len(t, closures, expected.closureCount)

	actualDepthCounts := make(map[uint32]int)
	for _, closure := range closures {
		require.Equal(t, model.RelationStatusActive, closure.Status)
		actualDepthCounts[closure.Depth]++
		if closure.Depth == 0 {
			require.Equal(t, closure.AncestorAgentID, closure.DescendantAgentID)
			require.Equal(t, model.RelationTypeClosure, closure.RelationType)
			require.Equal(t, jsonUint(closure.AncestorAgentID), closure.PathSnapshot)
			require.Nil(t, closure.ViaDirectParentID)
			continue
		}
		require.NotNil(t, closure.ViaDirectParentID)
		if closure.Depth == 1 {
			require.Equal(t, model.RelationTypeDirect, closure.RelationType)
			require.Equal(t, closure.AncestorAgentID, *closure.ViaDirectParentID)
			require.Equal(t, jsonUint(closure.AncestorAgentID)+"/"+jsonUint(closure.DescendantAgentID), closure.PathSnapshot)
			continue
		}
		require.Equal(t, model.RelationTypeClosure, closure.RelationType)
	}
	if expected.depthCounts != nil {
		require.Equal(t, expected.depthCounts, actualDepthCounts)
	}

	for _, agentID := range expected.selfClosures {
		closure := findClosure(t, closures, agentID, agentID, 0)
		require.Equal(t, model.RelationTypeClosure, closure.RelationType)
		require.Nil(t, closure.ViaDirectParentID)
	}

	for _, item := range expected.directClosures {
		assertClosureExpectation(t, closures, closureExpectation{
			ancestorID:        item.ancestorID,
			descendantID:      item.descendantID,
			depth:             1,
			viaDirectParentID: item.viaDirectParentID,
			relationType:      item.relationType,
		})
	}

	for _, item := range expected.indirectClosures {
		assertClosureExpectation(t, closures, item)
	}

	for _, item := range expected.missingClosures {
		assertClosureMissing(t, closures, item)
	}
}

func assertClosureExpectation(t *testing.T, closures []model.AgentRelationClosure, expected closureExpectation) {
	t.Helper()
	closure := findClosure(t, closures, expected.ancestorID, expected.descendantID, expected.depth)
	require.Equal(t, expected.relationType, closure.RelationType)
	if expected.pathSnapshot != "" {
		require.Equal(t, expected.pathSnapshot, closure.PathSnapshot)
	}
	if expected.viaDirectParentID == nil {
		require.Nil(t, closure.ViaDirectParentID)
		return
	}
	require.NotNil(t, closure.ViaDirectParentID)
	require.Equal(t, *expected.viaDirectParentID, *closure.ViaDirectParentID)
}

func assertClosureMissing(t *testing.T, closures []model.AgentRelationClosure, missing closureKey) {
	t.Helper()
	for _, closure := range closures {
		if closure.AncestorAgentID == missing.ancestorID && closure.DescendantAgentID == missing.descendantID && closure.Depth == missing.depth {
			t.Fatalf("unexpected closure found ancestor=%d descendant=%d depth=%d", missing.ancestorID, missing.descendantID, missing.depth)
		}
	}
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func createRuleSnapshotFixture(t *testing.T, db *gorm.DB, fixture ruleSnapshotFixture) model.RuleSnapshot {
	t.Helper()
	payload := model.CommissionRule{
		BaseModel:          model.BaseModel{ID: fixture.ruleID},
		Priority:           fixture.priority,
		Version:            fixture.ruleVersion,
		Scope:              fixture.scope,
		AgentID:            fixture.agentID,
		GameID:             fixture.gameID,
		EffectiveFrom:      fixture.effectiveFrom,
		PublishedAt:        &fixture.publishedAt,
		RuleName:           fixture.ruleUniqueKey,
		Status:             model.RuleStatusPublished,
		Currency:           "CNY",
		MaxSettlementDepth: 1,
	}
	if fixture.effectiveTo != nil {
		payload.EffectiveTo = fixture.effectiveTo
	}
	payloadJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	snapshot := model.RuleSnapshot{
		RuleID:          fixture.ruleID,
		RuleVersion:     fixture.ruleVersion,
		RuleUniqueKey:   fixture.ruleUniqueKey,
		Scope:           fixture.scope,
		AgentID:         fixture.agentID,
		GameID:          fixture.gameID,
		SnapshotHash:    fixture.ruleUniqueKey + "-hash",
		SnapshotPayload: datatypes.JSON(payloadJSON),
		EffectiveFrom:   fixture.effectiveFrom,
		EffectiveTo:     fixture.effectiveTo,
		PublishedAt:     fixture.publishedAt,
		PublishedBy:     "test",
	}
	require.NoError(t, db.Create(&snapshot).Error)
	return snapshot
}

type ruleSnapshotFixture struct {
	ruleID        uint64
	ruleVersion   uint32
	ruleUniqueKey string
	scope         model.RuleScope
	agentID       *uint64
	gameID        *uint64
	priority      int32
	effectiveFrom time.Time
	effectiveTo   *time.Time
	publishedAt   time.Time
}

func TestAgentGameAccessEndpoints(t *testing.T) {

	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-GAME", "Game Agent")
	gameA := createGameFixture(t, testDB, "GAME-A", true)
	gameB := createGameFixture(t, testDB, "GAME-B", false)

	listEmpty := performRequestWithToken(t, router, "operator", http.MethodGet, "/api/agent-game-access", nil)
	require.Equal(t, http.StatusOK, listEmpty.Code)
	var emptyBody struct {
		Items []struct{} `json:"items"`
		Total int        `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listEmpty.Body.Bytes(), &emptyBody))
	require.Equal(t, 0, emptyBody.Total)

	createResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agent-game-access", map[string]any{
		"agentID": agent.ID,
		"gameID":  gameA.ID,
		"status":  "enabled",
		"remark":  "phase2 grant",
	})
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	var createdAccess model.AgentGameAccess
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &createdAccess))
	require.Equal(t, agent.ID, createdAccess.AgentID)
	require.Equal(t, gameA.ID, createdAccess.GameID)
	require.Equal(t, model.AccessStatusEnabled, createdAccess.Status)
	require.Equal(t, "admin", createdAccess.GrantedBy)
	if createdAccess.SettlementMemo != "phase2 grant" {
		require.Equal(t, "phase2 grant", createdAccess.SettlementMemo)
	}

	listResp := performRequestWithToken(t, router, "operator", http.MethodGet, "/api/agent-game-access?agentID="+jsonUint(agent.ID), nil)
	require.Equal(t, http.StatusOK, listResp.Code)
	var listBody struct {
		Items []struct {
			model.AgentGameAccess
			AgentName string `json:"agentName"`
			GameCode  string `json:"gameCode"`
			GameName  string `json:"gameName"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listBody))
	require.Equal(t, 1, listBody.Total)
	require.Equal(t, agent.Name, listBody.Items[0].AgentName)
	require.Equal(t, gameA.GameCode, listBody.Items[0].GameCode)
	require.Equal(t, gameA.Name, listBody.Items[0].GameName)

	updateResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agent-game-access", map[string]any{
		"agentID": agent.ID,
		"gameID":  gameA.ID,
		"status":  "disabled",
		"remark":  "phase2 revoke pending",
	})
	require.Equal(t, http.StatusCreated, updateResp.Code, updateResp.Body.String())
	var updatedAccess model.AgentGameAccess
	require.NoError(t, json.Unmarshal(updateResp.Body.Bytes(), &updatedAccess))
	require.Equal(t, createdAccess.ID, updatedAccess.ID)
	require.Equal(t, model.AccessStatusDisabled, updatedAccess.Status)
	require.Equal(t, "phase2 revoke pending", updatedAccess.SettlementMemo)

	forbiddenCreate := performJSONWithToken(t, router, "operator", http.MethodPost, "/api/agent-game-access", map[string]any{
		"agentID": agent.ID,
		"gameID":  gameA.ID,
	})
	require.Equal(t, http.StatusForbidden, forbiddenCreate.Code)
	assertJSONErrorBody(t, forbiddenCreate, "forbidden")

	nonexistentGameResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agent-game-access", map[string]any{
		"agentID": agent.ID,
		"gameID":  uint64(999999),
	})
	require.Equal(t, http.StatusNotFound, nonexistentGameResp.Code)
	assertJSONErrorBody(t, nonexistentGameResp, "record not found")

	nonAgentableResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/agent-game-access", map[string]any{
		"agentID": agent.ID,
		"gameID":  gameB.ID,
	})
	require.Equal(t, http.StatusCreated, nonAgentableResp.Code, nonAgentableResp.Body.String())

	deleteResp := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/agent-game-access?agentID="+jsonUint(agent.ID)+"&gameID="+jsonUint(gameA.ID), nil)
	require.Equal(t, http.StatusNoContent, deleteResp.Code)

	missingDelete := performRequestWithToken(t, router, "admin", http.MethodDelete, "/api/agent-game-access?agentID="+jsonUint(agent.ID)+"&gameID="+jsonUint(gameA.ID), nil)
	require.Equal(t, http.StatusNotFound, missingDelete.Code)

	var auditLogs []model.OperationAuditLog
	require.NoError(t, testDB.Where("target_type = ?", "agent_game_access").Order("id asc").Find(&auditLogs).Error)
	require.GreaterOrEqual(t, len(auditLogs), 4)
	foundSuccessCreate := false
	foundSuccessDelete := false
	for _, entry := range auditLogs {
		if entry.Action == "agent_game_access_upsert" && entry.Result == model.AuditResultSuccess {
			foundSuccessCreate = true
		}
		if entry.Action == "agent_game_access_revoke" && entry.Result == model.AuditResultSuccess {
			foundSuccessDelete = true
		}
	}
	require.True(t, foundSuccessCreate)
	require.True(t, foundSuccessDelete)
}

func createGameFixture(t *testing.T, db *gorm.DB, code string, isAgentable bool) model.Game {
	t.Helper()
	game := model.Game{GameCode: code, Name: code, Vendor: "vendor", Category: "slot", Status: model.GameStatusOnline, IsAgentable: isAgentable}
	require.NoError(t, db.Create(&game).Error)
	return game
}

func TestSettlementBillGenerateConfirmSupportsDailyWeeklyMonthlyCycles(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	type cycleCase struct {
		name             string
		cycle            string
		anchor           time.Time
		wantStart        time.Time
		wantEnd          time.Time
		commissions      []float64
		adjustment       float64
		wantCommission   float64
		wantPayable      float64
		wantSummaryCycle string
	}

	cases := []cycleCase{
		{
			name:             "daily",
			cycle:            "daily",
			anchor:           time.Date(2026, 4, 10, 15, 30, 0, 0, time.UTC),
			wantStart:        time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
			wantEnd:          time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
			commissions:      []float64{10.25, 4.75},
			adjustment:       1.25,
			wantCommission:   15.0,
			wantPayable:      16.25,
			wantSummaryCycle: "daily",
		},
		{
			name:             "weekly",
			cycle:            "weekly",
			anchor:           time.Date(2026, 4, 15, 9, 0, 0, 0, time.UTC),
			wantStart:        time.Date(2026, 4, 13, 0, 0, 0, 0, time.UTC),
			wantEnd:          time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
			commissions:      []float64{7.5, 2.25},
			adjustment:       -0.75,
			wantCommission:   9.75,
			wantPayable:      9.0,
			wantSummaryCycle: "weekly",
		},
		{
			name:             "monthly",
			cycle:            "monthly",
			anchor:           time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
			wantStart:        time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			wantEnd:          time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
			commissions:      []float64{21.0, 3.5},
			adjustment:       2.0,
			wantCommission:   24.5,
			wantPayable:      26.5,
			wantSummaryCycle: "monthly",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := createAgentFixture(t, testDB, "AG-CYCLE-"+strings.ToUpper(tc.cycle), "Cycle Agent "+tc.cycle)
			require.NoError(t, testDB.Model(&agent).Update("remark", "P3-T1").Error)
			require.NoError(t, testDB.First(&agent, agent.ID).Error)
			records := make([]model.CommissionRecord, 0, len(tc.commissions)+1)
			for i, amount := range tc.commissions {
				paidAt := tc.wantStart.Add(time.Duration(i+1) * time.Hour)
				callbackAt := tc.wantStart.Add(time.Duration(i+1) * time.Hour)
				estimatedAt := tc.wantStart.Add(time.Duration(i+2) * time.Hour)
				order := model.RechargeOrder{
					OrderNo:         "ORD-CYCLE-" + strings.ToUpper(tc.cycle) + "-" + strconv.Itoa(i+1),
					ExternalOrderNo: "EXT-CYCLE-" + strings.ToUpper(tc.cycle) + "-" + strconv.Itoa(i+1),
					Amount:          100 + float64(i),
					Currency:        "CNY",
					Status:          model.OrderStatusPaid,
					PaidAt:          &paidAt,
					CallbackAt:      &callbackAt,
					Channel:         "test",
					IdempotencyKey:  "IDEMP-CYCLE-" + strings.ToUpper(tc.cycle) + "-" + strconv.Itoa(i+1),
				}
				require.NoError(t, testDB.Create(&order).Error)
				records = append(records, model.CommissionRecord{
					RecordNo:             "COM-CYCLE-" + strings.ToUpper(tc.cycle) + "-" + strconv.Itoa(i+1),
					AgentID:              agent.ID,
					RechargeOrderID:      order.ID,
					CommissionRate:       0.1,
					CommissionAmount:     amount,
					CommissionBaseAmount: amount * 10,
					Currency:             "CNY",
					SettlementDepth:      1,
					Status:               model.CommissionStatusPending,
					EstimatedAt:          estimatedAt,
					Remark:               tc.cycle + " in-range",
				})
			}
			outOfRangePaidAt := tc.wantEnd
			outOfRangeCallbackAt := tc.wantEnd
			outOfRangeOrder := model.RechargeOrder{
				OrderNo:         "ORD-CYCLE-" + strings.ToUpper(tc.cycle) + "-OUT",
				ExternalOrderNo: "EXT-CYCLE-" + strings.ToUpper(tc.cycle) + "-OUT",
				Amount:          999,
				Currency:        "CNY",
				Status:          model.OrderStatusPaid,
				PaidAt:          &outOfRangePaidAt,
				CallbackAt:      &outOfRangeCallbackAt,
				Channel:         "test",
				IdempotencyKey:  "IDEMP-CYCLE-" + strings.ToUpper(tc.cycle) + "-OUT",
			}
			require.NoError(t, testDB.Create(&outOfRangeOrder).Error)
			records = append(records, model.CommissionRecord{
				RecordNo:             "COM-CYCLE-" + strings.ToUpper(tc.cycle) + "-OUT",
				AgentID:              agent.ID,
				RechargeOrderID:      outOfRangeOrder.ID,
				CommissionRate:       0.1,
				CommissionAmount:     100,
				CommissionBaseAmount: 1000,
				Currency:             "CNY",
				SettlementDepth:      1,
				Status:               model.CommissionStatusPending,
				EstimatedAt:          tc.wantEnd,
				Remark:               tc.cycle + " out-of-range",
			})
			require.NoError(t, testDB.Create(&records).Error)

			generateResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/generate", map[string]any{
				"agentID":     agent.ID,
				"cycle":       tc.cycle,
				"periodStart": tc.anchor.Format(time.RFC3339),
				"currency":    "CNY",
				"adjustment":  tc.adjustment,
				"remark":      tc.cycle + " cycle bill",
			})
			require.Equal(t, http.StatusCreated, generateResp.Code, generateResp.Body.String())

			var generated struct {
				model.SettlementBill
				Details []struct {
					model.SettlementBillDetail
					RecordNo string `json:"recordNo"`
					OrderNo  string `json:"orderNo"`
				} `json:"details"`
			}
			require.NoError(t, json.Unmarshal(generateResp.Body.Bytes(), &generated))
			require.Equal(t, tc.wantStart, generated.PeriodStart.UTC())
			require.Equal(t, tc.wantEnd, generated.PeriodEnd.UTC())
			require.Equal(t, model.SettlementBillStatusGenerated, generated.Status)
			adjustment := math.Round(tc.adjustment*100) / 100
			payable := math.Round(tc.wantPayable*100) / 100
			require.InDelta(t, tc.wantCommission, generated.CommissionAmount, 0.001)
			require.InDelta(t, adjustment, generated.AdjustmentAmount, 0.011)
			require.InDelta(t, payable, generated.PayableAmount, 0.011)
			require.Len(t, generated.Details, len(tc.commissions))

			var summary map[string]any
			require.NoError(t, json.Unmarshal(generated.SummaryPayload, &summary))
			require.Equal(t, tc.wantSummaryCycle, summary["cycle"])
			require.Equal(t, float64(len(tc.commissions)), summary["recordCount"])

			confirmResp := performRequestWithToken(t, router, "admin", http.MethodPost, "/api/settlement-bills/"+jsonUint(generated.ID)+"/confirm", nil)
			require.Equal(t, http.StatusOK, confirmResp.Code, confirmResp.Body.String())
			var confirmed model.SettlementBill
			require.NoError(t, json.Unmarshal(confirmResp.Body.Bytes(), &confirmed))
			require.Equal(t, model.SettlementBillStatusConfirmed, confirmed.Status)
			require.NotNil(t, confirmed.ConfirmedAt)
			require.Equal(t, "admin", confirmed.ConfirmedBy)
			require.Equal(t, tc.wantStart, confirmed.PeriodStart.UTC())
			require.Equal(t, tc.wantEnd, confirmed.PeriodEnd.UTC())
		})
	}
}

func TestSettlementBillGenerateRejectsMismatchedDerivedCycleEnd(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-CYCLE-BAD", "Cycle Agent Bad")

	resp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/generate", map[string]any{
		"agentID":     agent.ID,
		"cycle":       "weekly",
		"periodStart": "2026-04-15T09:00:00Z",
		"periodEnd":   "2026-04-19T00:00:00Z",
	})
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assertJSONErrorBody(t, resp, "periodEnd does not match weekly cycle derived end")
}

func TestSettlementBillsAndRecalculationTaskEndpoints(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-SETTLE", "Settlement Agent")
	require.NoError(t, testDB.Model(&agent).Update("remark", "P3-T1").Error)
	require.NoError(t, testDB.First(&agent, agent.ID).Error)
	paidAtA := time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC)
	paidAtB := paidAtA.Add(2 * time.Hour)
	periodStart := paidAtA.Add(-1 * time.Hour)
	periodEnd := paidAtB.Add(2 * time.Hour)

	orderA := model.RechargeOrder{
		OrderNo:         "ORD-SET-1",
		ExternalOrderNo: "EXT-SET-1",
		PlayerID:        1001,
		GameID:          2001,
		Amount:          120,
		Currency:        "CNY",
		Status:          model.OrderStatusPaid,
		PaidAt:          &paidAtA,
		IdempotencyKey:  "settlement-bill-order-a",
	}
	orderB := model.RechargeOrder{
		OrderNo:         "ORD-SET-2",
		ExternalOrderNo: "EXT-SET-2",
		PlayerID:        1002,
		GameID:          2002,
		Amount:          80,
		Currency:        "CNY",
		Status:          model.OrderStatusPaid,
		PaidAt:          &paidAtB,
		IdempotencyKey:  "settlement-bill-order-b",
	}
	require.NoError(t, testDB.Create(&orderA).Error)
	require.NoError(t, testDB.Create(&orderB).Error)

	records := []model.CommissionRecord{
		{
			RecordNo:             "COM-SET-1",
			RechargeOrderID:      orderA.ID,
			PlayerID:             orderA.PlayerID,
			AgentID:              agent.ID,
			GameID:               orderA.GameID,
			SettlementDepth:      1,
			CommissionBaseAmount: orderA.Amount,
			CommissionRate:       0.1,
			CommissionAmount:     12.34,
			Currency:             "CNY",
			Status:               model.CommissionStatusPending,
			EstimatedAt:          paidAtA,
			Remark:               "first commission",
		},
		{
			RecordNo:             "COM-SET-2",
			RechargeOrderID:      orderB.ID,
			PlayerID:             orderB.PlayerID,
			AgentID:              agent.ID,
			GameID:               orderB.GameID,
			SettlementDepth:      1,
			CommissionBaseAmount: orderB.Amount,
			CommissionRate:       0.1,
			CommissionAmount:     7.66,
			Currency:             "CNY",
			Status:               model.CommissionStatusPending,
			EstimatedAt:          paidAtB,
			Remark:               "second commission",
		},
	}
	require.NoError(t, testDB.Create(&records).Error)

	generateResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/generate", map[string]any{
		"agentID":     agent.ID,
		"periodStart": periodStart.Format(time.RFC3339),
		"periodEnd":   periodEnd.Format(time.RFC3339),
		"currency":    "CNY",
		"adjustment":  1.5,
		"remark":      "phase 2 bill",
	})
	require.Equal(t, http.StatusCreated, generateResp.Code, generateResp.Body.String())
	var generatedBill struct {
		model.SettlementBill
		AgentName string `json:"agentName"`
		Details   []struct {
			model.SettlementBillDetail
			RecordNo string `json:"recordNo"`
			OrderNo  string `json:"orderNo"`
		} `json:"details"`
	}
	require.NoError(t, json.Unmarshal(generateResp.Body.Bytes(), &generatedBill))
	require.NotZero(t, generatedBill.ID)
	require.Equal(t, agent.ID, generatedBill.AgentID)
	require.Equal(t, agent.Name, generatedBill.AgentName)
	require.Equal(t, model.SettlementBillStatusGenerated, generatedBill.Status)
	require.InDelta(t, 20.0, generatedBill.CommissionAmount, 0.001)
	require.InDelta(t, 1.5, generatedBill.AdjustmentAmount, 0.001)
	require.InDelta(t, 21.5, generatedBill.PayableAmount, 0.001)
	require.Len(t, generatedBill.Details, 2)
	require.Equal(t, "COM-SET-1", generatedBill.Details[0].RecordNo)
	require.Equal(t, "ORD-SET-1", generatedBill.Details[0].OrderNo)
	require.Equal(t, "COM-SET-2", generatedBill.Details[1].RecordNo)
	require.Equal(t, "ORD-SET-2", generatedBill.Details[1].OrderNo)

	listResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/settlement-bills?agentID="+jsonUint(agent.ID)+"&status=generated", nil)
	require.Equal(t, http.StatusOK, listResp.Code)
	var billList struct {
		Items []struct {
			model.SettlementBill
			AgentName string `json:"agentName"`
			Details   []struct {
				RecordNo string `json:"recordNo"`
				OrderNo  string `json:"orderNo"`
			} `json:"details"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &billList))
	require.Equal(t, 1, billList.Total)
	require.Equal(t, generatedBill.ID, billList.Items[0].ID)
	require.Equal(t, agent.Name, billList.Items[0].AgentName)
	require.Len(t, billList.Items[0].Details, 2)

	filteredByBillNoResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/settlement-bills?billNo="+generatedBill.BillNo, nil)
	require.Equal(t, http.StatusOK, filteredByBillNoResp.Code)
	var filteredBillList struct {
		Items []struct {
			model.SettlementBill
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(filteredByBillNoResp.Body.Bytes(), &filteredBillList))
	require.Equal(t, 1, filteredBillList.Total)
	require.Equal(t, generatedBill.ID, filteredBillList.Items[0].ID)

	missingBillNoResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/settlement-bills?billNo=SB-NOT-FOUND", nil)
	require.Equal(t, http.StatusOK, missingBillNoResp.Code)
	require.NoError(t, json.Unmarshal(missingBillNoResp.Body.Bytes(), &filteredBillList))
	require.Zero(t, filteredBillList.Total)

	confirmResp := performRequestWithToken(t, router, "admin", http.MethodPost, "/api/settlement-bills/"+jsonUint(generatedBill.ID)+"/confirm", nil)
	require.Equal(t, http.StatusOK, confirmResp.Code, confirmResp.Body.String())
	var confirmedBill struct {
		model.SettlementBill
		AgentName string `json:"agentName"`
		Details   []struct {
			model.SettlementBillDetail
			RecordNo string `json:"recordNo"`
			OrderNo  string `json:"orderNo"`
		} `json:"details"`
	}
	require.NoError(t, json.Unmarshal(confirmResp.Body.Bytes(), &confirmedBill))
	require.Equal(t, generatedBill.ID, confirmedBill.ID)
	require.Equal(t, model.SettlementBillStatusConfirmed, confirmedBill.Status)
	require.Equal(t, agent.Name, confirmedBill.AgentName)
	require.Len(t, confirmedBill.Details, 2)
	require.NotNil(t, confirmedBill.ConfirmedAt)
	require.Equal(t, "admin", confirmedBill.ConfirmedBy)

	var storedConfirmedBill model.SettlementBill
	require.NoError(t, testDB.First(&storedConfirmedBill, generatedBill.ID).Error)
	require.Equal(t, model.SettlementBillStatusConfirmed, storedConfirmedBill.Status)
	require.NotNil(t, storedConfirmedBill.ConfirmedAt)
	require.Equal(t, "admin", storedConfirmedBill.ConfirmedBy)

	var confirmAudit model.OperationAuditLog
	require.NoError(t, testDB.Where("action = ? AND target_id = ?", "settlement_bill_confirm", jsonUint(generatedBill.ID)).First(&confirmAudit).Error)
	require.Equal(t, model.AuditModuleCommission, confirmAudit.Module)

	exportResp := performRequestWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/"+jsonUint(generatedBill.ID)+"/export", nil)
	require.Equal(t, http.StatusNoContent, exportResp.Code, exportResp.Body.String())

	var exportedBill model.SettlementBill
	require.NoError(t, testDB.First(&exportedBill, generatedBill.ID).Error)
	var summary map[string]any
	require.NoError(t, json.Unmarshal(exportedBill.SummaryPayload, &summary))
	require.NotEmpty(t, summary["exportedAt"])

	var exportAudit model.OperationAuditLog
	require.NoError(t, testDB.Where("action = ? AND target_id = ?", "settlement_bill_export", jsonUint(generatedBill.ID)).First(&exportAudit).Error)
	require.Equal(t, model.AuditModuleCommission, exportAudit.Module)

	taskResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{
		"taskType":         "settlement_bill",
		"agentID":          agent.ID,
		"settlementBillID": generatedBill.ID,
		"periodStart":      periodStart.Format(time.RFC3339),
		"periodEnd":        periodEnd.Format(time.RFC3339),
		"remark":           "rerun settlement",
	})
	require.Equal(t, http.StatusCreated, taskResp.Code, taskResp.Body.String())
	var createdTask struct {
		model.RecalculationTask
		AgentName string `json:"agentName"`
		BillNo    string `json:"billNo"`
	}
	require.NoError(t, json.Unmarshal(taskResp.Body.Bytes(), &createdTask))
	require.NotZero(t, createdTask.ID)
	require.Equal(t, model.RecalculationTaskStatusCompleted, createdTask.Status)
	require.Equal(t, agent.ID, *createdTask.AgentID)
	require.Equal(t, generatedBill.ID, *createdTask.SettlementBillID)
	require.Equal(t, agent.Name, createdTask.AgentName)
	require.Equal(t, generatedBill.BillNo, createdTask.BillNo)
	require.Equal(t, "finance", createdTask.RequestedBy)
	require.NotNil(t, createdTask.StartedAt)
	require.NotNil(t, createdTask.CompletedAt)
	require.NotEmpty(t, createdTask.ResultSummary)

	var rerunBill model.SettlementBill
	require.NoError(t, testDB.Where("source_task_id = ?", createdTask.ID).First(&rerunBill).Error)
	require.NotZero(t, rerunBill.ID)
	require.NotEqual(t, generatedBill.ID, rerunBill.ID)
	require.Equal(t, generatedBill.AgentID, rerunBill.AgentID)
	require.Equal(t, generatedBill.PeriodStart.UTC(), rerunBill.PeriodStart.UTC())
	require.Equal(t, generatedBill.PeriodEnd.UTC(), rerunBill.PeriodEnd.UTC())
	require.InDelta(t, generatedBill.CommissionAmount, rerunBill.CommissionAmount, 0.001)
	require.InDelta(t, 0.0, rerunBill.AdjustmentAmount, 0.001)
	require.InDelta(t, generatedBill.CommissionAmount, rerunBill.PayableAmount, 0.001)
	require.Equal(t, model.SettlementBillStatusGenerated, rerunBill.Status)

	var rerunDetails []model.SettlementBillDetail
	require.NoError(t, testDB.Where("settlement_bill_id = ?", rerunBill.ID).Order("id asc").Find(&rerunDetails).Error)
	require.Len(t, rerunDetails, 2)
	require.Equal(t, "COM-SET-1", rerunDetails[0].ReferenceID)
	require.Equal(t, "COM-SET-2", rerunDetails[1].ReferenceID)

	var resultSummary map[string]any
	require.NoError(t, json.Unmarshal(createdTask.ResultSummary, &resultSummary))
	require.Equal(t, "completed", resultSummary["result"])
	require.Equal(t, "settlement_bill", resultSummary["taskType"])
	require.Equal(t, float64(rerunBill.ID), resultSummary["generatedSettlementBillID"])
	require.Equal(t, float64(2), resultSummary["detailCount"])
	require.Equal(t, float64(20), resultSummary["commissionAmount"])

	listTasksResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/recalculation-tasks?status=completed", nil)
	require.Equal(t, http.StatusOK, listTasksResp.Code)
	var taskList struct {
		Items []struct {
			model.RecalculationTask
			AgentName string `json:"agentName"`
			BillNo    string `json:"billNo"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listTasksResp.Body.Bytes(), &taskList))
	require.Equal(t, 1, taskList.Total)
	require.Equal(t, createdTask.ID, taskList.Items[0].ID)
	require.Equal(t, agent.Name, taskList.Items[0].AgentName)
	require.Equal(t, generatedBill.BillNo, taskList.Items[0].BillNo)
}

func TestSettlementBillAndRecalculationValidation(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-VALIDATE", "Validate Agent")

	badBill := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/generate", map[string]any{
		"agentID":     agent.ID,
		"periodStart": "2026-04-02T00:00:00Z",
		"periodEnd":   "2026-04-01T00:00:00Z",
	})
	require.Equal(t, http.StatusBadRequest, badBill.Code)
	assertJSONErrorBody(t, badBill, "periodEnd must be after periodStart")

	missingAgentBill := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/generate", map[string]any{
		"agentID":     uint64(999999),
		"periodStart": "2026-04-01T00:00:00Z",
		"periodEnd":   "2026-04-02T00:00:00Z",
	})
	require.Equal(t, http.StatusNotFound, missingAgentBill.Code)

	invalidList := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/settlement-bills?agentID=bad-id", nil)
	require.Equal(t, http.StatusBadRequest, invalidList.Code)

	badTask := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{
		"agentID":     agent.ID,
		"periodStart": "2026-04-03T00:00:00Z",
		"periodEnd":   "2026-04-01T00:00:00Z",
	})
	require.Equal(t, http.StatusBadRequest, badTask.Code)
	assertJSONErrorBody(t, badTask, "periodEnd must be after periodStart")

	missingScopeTask := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{
		"agentID": agent.ID,
	})
	require.Equal(t, http.StatusBadRequest, missingScopeTask.Code)
	assertJSONErrorBody(t, missingScopeTask, "periodStart and periodEnd are required")

	unsupportedTaskType := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{
		"taskType":    "unsupported",
		"agentID":     agent.ID,
		"periodStart": "2026-04-01T00:00:00Z",
		"periodEnd":   "2026-04-02T00:00:00Z",
	})
	require.Equal(t, http.StatusBadRequest, unsupportedTaskType.Code)
	assertJSONErrorBody(t, unsupportedTaskType, "taskType unsupported is not supported")

	missingBillTask := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{
		"settlementBillID": uint64(999999),
	})
	require.Equal(t, http.StatusNotFound, missingBillTask.Code)

	invalidConfirmID := performRequestWithToken(t, router, "admin", http.MethodPost, "/api/settlement-bills/bad-id/confirm", nil)
	require.Equal(t, http.StatusBadRequest, invalidConfirmID.Code)

	missingConfirm := performRequestWithToken(t, router, "admin", http.MethodPost, "/api/settlement-bills/999999/confirm", nil)
	require.Equal(t, http.StatusNotFound, missingConfirm.Code)

	invalidExportID := performRequestWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/bad-id/export", nil)
	require.Equal(t, http.StatusBadRequest, invalidExportID.Code)

	missingExport := performRequestWithToken(t, router, "finance", http.MethodPost, "/api/settlement-bills/999999/export", nil)
	require.Equal(t, http.StatusNotFound, missingExport.Code)
}

func TestRecalculationTaskCapturesOperatorReasonAndScope(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-TASK-META", "Task Meta Agent")

	periodStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(24 * time.Hour)

	resp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{
		"taskType":    string(model.RecalculationTaskTypeSettlementBill),
		"agentID":     agent.ID,
		"periodStart": periodStart.Format(time.RFC3339),
		"periodEnd":   periodEnd.Format(time.RFC3339),
		"operator":    "alice",
		"reason":      "refund correction",
		"scope":       "agent_period",
		"remark":      "manual verification",
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var created model.RecalculationTask
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &created))
	require.Equal(t, "alice", created.RequestedBy)
	require.Contains(t, created.Remark, "operator=alice")
	require.Contains(t, created.Remark, "reason=refund correction")
	require.Contains(t, created.Remark, "scope=agent_period")
	require.Contains(t, created.Remark, "manual verification")

	var stored model.RecalculationTask
	require.NoError(t, testDB.First(&stored, created.ID).Error)
	require.Equal(t, "alice", stored.RequestedBy)
	require.Contains(t, stored.Remark, "operator=alice")
	require.Contains(t, stored.Remark, "reason=refund correction")
	require.Contains(t, stored.Remark, "scope=agent_period")
	require.Contains(t, stored.Remark, "manual verification")
}

func TestCommissionRecalculationTaskEndpoints(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-COMM-REC", "Commission Recalc Agent")
	account := createAgentAccountFixture(t, testDB, agent.ID, "ACC-COMM-REC")
	account.Balance = 10
	account.WithdrawableAmount = 10
	require.NoError(t, testDB.Save(&account).Error)

	paidAt := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	game := createGameFixture(t, testDB, "GAME-COMM-REC", true)
	require.NoError(t, testDB.Create(&model.AgentGameAccess{
		AgentID:       agent.ID,
		GameID:        game.ID,
		Status:        model.AccessStatusEnabled,
		GrantedBy:     "admin",
		GrantedAt:     paidAt.Add(-2 * time.Hour),
		EffectiveFrom: paidAt.Add(-2 * time.Hour),
	}).Error)

	player := model.Player{
		PlayerNo:       "PLY-COMM-REC",
		PlatformUserID: "platform-comm-rec",
		Nickname:       "commission-player",
		Currency:       "CNY",
		Status:         "active",
		RegisteredAt:   paidAt.Add(-24 * time.Hour),
	}
	require.NoError(t, testDB.Create(&player).Error)

	binding := model.Binding{
		PlayerID:      player.ID,
		AgentID:       agent.ID,
		Status:        model.BindingStatusBound,
		Source:        model.BindingSourceRegister,
		BoundAt:       paidAt.Add(-24 * time.Hour),
		EffectiveFrom: paidAt.Add(-24 * time.Hour),
		ApprovedBy:    "admin",
	}
	require.NoError(t, testDB.Create(&binding).Error)

	rule := model.CommissionRule{
		RuleName:           "commission recalc rule",
		Scope:              model.RuleScopeAgentGame,
		RuleType:           model.RuleTypeRatio,
		Status:             model.RuleStatusPublished,
		Priority:           100,
		Version:            1,
		AgentID:            &agent.ID,
		GameID:             &game.ID,
		MaxSettlementDepth: 1,
		CommissionRate:     0.20,
		Currency:           "CNY",
		EffectiveFrom:      paidAt.Add(-2 * time.Hour),
		PublishedAt:        ptrTime(paidAt.Add(-time.Hour)),
		PublishedBy:        "admin",
		UniqueKey:          "agent_game|commission-recalc",
	}
	require.NoError(t, testDB.Create(&rule).Error)
	rulePayload, err := json.Marshal(rule)
	require.NoError(t, err)
	snapshot := model.RuleSnapshot{
		RuleID:          rule.ID,
		RuleVersion:     rule.Version,
		RuleUniqueKey:   rule.UniqueKey,
		Scope:           rule.Scope,
		AgentID:         rule.AgentID,
		GameID:          rule.GameID,
		SnapshotHash:    "snapshot-commission-recalc-1",
		SnapshotPayload: rulePayload,
		EffectiveFrom:   rule.EffectiveFrom,
		PublishedAt:     paidAt.Add(-time.Hour),
		PublishedBy:     "admin",
	}
	require.NoError(t, testDB.Create(&snapshot).Error)

	order := model.RechargeOrder{
		OrderNo:         "ORD-COMM-REC-1",
		ExternalOrderNo: "EXT-COMM-REC-1",
		PlayerID:        player.ID,
		GameID:          game.ID,
		Amount:          100,
		Currency:        "CNY",
		Status:          model.OrderStatusPaid,
		PaidAt:          &paidAt,
		AgentID:         &agent.ID,
		BindingID:       &binding.ID,
		RuleSnapshotID:  &snapshot.ID,
		IdempotencyKey:  "commission-recalc-order-1",
	}
	require.NoError(t, testDB.Create(&order).Error)

	existingRecord := model.CommissionRecord{
		RecordNo:             "COM-COMM-REC-1",
		RechargeOrderID:      order.ID,
		PlayerID:             player.ID,
		AgentID:              agent.ID,
		GameID:               game.ID,
		RuleID:               &rule.ID,
		RuleSnapshotID:       &snapshot.ID,
		SettlementDepth:      1,
		CommissionBaseAmount: order.Amount,
		CommissionRate:       0.10,
		CommissionAmount:     10,
		Currency:             "CNY",
		Status:               model.CommissionStatusSettled,
		EstimatedAt:          paidAt,
		SettledAt:            ptrTime(paidAt),
		Remark:               "stale commission",
	}
	require.NoError(t, testDB.Create(&existingRecord).Error)

	periodStart := paidAt.Add(-time.Hour)
	periodEnd := paidAt.Add(time.Hour)
	resp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recalculation-tasks", map[string]any{
		"taskType":    string(model.RecalculationTaskTypeCommission),
		"agentID":     agent.ID,
		"periodStart": periodStart.Format(time.RFC3339),
		"periodEnd":   periodEnd.Format(time.RFC3339),
		"remark":      "rerate commissions",
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var createdTask model.RecalculationTask
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &createdTask))
	require.Equal(t, model.RecalculationTaskTypeCommission, createdTask.TaskType)
	require.Equal(t, model.RecalculationTaskStatusCompleted, createdTask.Status)
	require.NotEmpty(t, createdTask.ResultSummary)
	require.Contains(t, createdTask.Remark, "rerate commissions")

	var persistedTask model.RecalculationTask
	require.NoError(t, testDB.First(&persistedTask, createdTask.ID).Error)
	require.Equal(t, createdTask.ID, persistedTask.ID)
	require.Contains(t, persistedTask.Remark, "rerate commissions")

	var updatedOriginal model.CommissionRecord
	require.NoError(t, testDB.First(&updatedOriginal, existingRecord.ID).Error)
	require.Equal(t, model.CommissionStatusReversed, updatedOriginal.Status)
	require.Contains(t, updatedOriginal.Remark, "recalculated")

	var records []model.CommissionRecord
	require.NoError(t, testDB.Where("recharge_order_id = ?", order.ID).Order("id asc").Find(&records).Error)
	require.Len(t, records, 3)
	require.Equal(t, existingRecord.ID, *records[1].ReversedFromID)
	require.Equal(t, float64(-10), records[1].CommissionAmount)
	require.Equal(t, model.CommissionStatusReversed, records[1].Status)
	require.Equal(t, float64(20), records[2].CommissionAmount)
	require.Equal(t, float64(0.2), records[2].CommissionRate)
	require.Equal(t, model.CommissionStatusSettled, records[2].Status)
	require.Equal(t, "recharge settlement", records[2].Remark)

	var refreshedAccount model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).First(&refreshedAccount).Error)
	require.InDelta(t, 20.0, refreshedAccount.Balance, 0.001)
	require.InDelta(t, 20.0, refreshedAccount.WithdrawableAmount, 0.001)

	var ledgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 2)
	require.Equal(t, model.LedgerDirectionDebit, ledgers[0].Direction)
	require.Equal(t, model.LedgerTypeReverse, ledgers[0].LedgerType)
	require.Equal(t, "recalculation_task", ledgers[0].ReferenceType)
	require.Equal(t, float64(10), ledgers[0].Amount)
	require.Equal(t, model.LedgerDirectionCredit, ledgers[1].Direction)
	require.Equal(t, model.LedgerTypeIncome, ledgers[1].LedgerType)

	var resultSummary map[string]any
	require.NoError(t, json.Unmarshal(createdTask.ResultSummary, &resultSummary))
	require.Equal(t, "completed", resultSummary["result"])
	require.Equal(t, "commission", resultSummary["taskType"])
	require.Equal(t, float64(1), resultSummary["recalculatedCount"])
	require.Equal(t, float64(1), resultSummary["createdRecordCount"])
	require.Equal(t, float64(1), resultSummary["impactedAccountCount"])
}

func TestRechargeCallbackRiskHitFreezesOrderWithoutDirectCredit(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-RISK-HIT", "Risk Hit Agent")
	account := createAgentAccountFixture(t, testDB, agent.ID, "ACC-RISK-HIT")
	account.Balance = 200
	account.WithdrawableAmount = 200
	require.NoError(t, testDB.Save(&account).Error)

	game := createGameFixture(t, testDB, "GAME-RISK-HIT", true)
	grantedAt := time.Date(2026, 4, 3, 9, 0, 0, 0, time.UTC)
	require.NoError(t, testDB.Create(&model.AgentGameAccess{
		AgentID:       agent.ID,
		GameID:        game.ID,
		Status:        model.AccessStatusEnabled,
		GrantedBy:     "admin",
		GrantedAt:     grantedAt,
		EffectiveFrom: grantedAt,
	}).Error)

	player := model.Player{
		PlayerNo:       "PLY-RISK-HIT",
		PlatformUserID: "platform-risk-hit",
		Nickname:       "risk-hit-player",
		Currency:       "CNY",
		Status:         "active",
		RegisteredAt:   grantedAt,
	}
	require.NoError(t, testDB.Create(&player).Error)
	require.NoError(t, testDB.Create(&model.Binding{
		PlayerID:      player.ID,
		AgentID:       agent.ID,
		Status:        model.BindingStatusBound,
		Source:        model.BindingSourceRegister,
		BoundAt:       grantedAt,
		EffectiveFrom: grantedAt,
		ApprovedBy:    "admin",
	}).Error)

	rule := model.CommissionRule{
		RuleName:           "risk hit rule",
		Scope:              model.RuleScopeAgentGame,
		RuleType:           model.RuleTypeRatio,
		Status:             model.RuleStatusPublished,
		Priority:           100,
		Version:            1,
		AgentID:            &agent.ID,
		GameID:             &game.ID,
		MaxSettlementDepth: 1,
		CommissionRate:     0.1,
		Currency:           "CNY",
		EffectiveFrom:      grantedAt.Add(-time.Hour),
		PublishedAt:        ptrTime(grantedAt),
		PublishedBy:        "admin",
		UniqueKey:          "agent_game|risk-hit",
	}
	require.NoError(t, testDB.Create(&rule).Error)
	rulePayload, err := json.Marshal(rule)
	require.NoError(t, err)
	snapshot := model.RuleSnapshot{
		RuleID:          rule.ID,
		RuleVersion:     rule.Version,
		RuleUniqueKey:   rule.UniqueKey,
		Scope:           rule.Scope,
		AgentID:         rule.AgentID,
		GameID:          rule.GameID,
		SnapshotHash:    "snapshot-risk-hit-1",
		SnapshotPayload: rulePayload,
		EffectiveFrom:   rule.EffectiveFrom,
		PublishedAt:     grantedAt,
		PublishedBy:     "admin",
	}
	require.NoError(t, testDB.Create(&snapshot).Error)

	riskResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
		"caseNo":   "RISK-HIT-001",
		"agentID":  agent.ID,
		"amount":   50.0,
		"currency": "CNY",
		"freeze":   true,
		"reason":   "manual review",
	})
	require.Equal(t, http.StatusCreated, riskResp.Code, riskResp.Body.String())

	paidAt := grantedAt.Add(time.Hour)
	callbackAt := paidAt.Add(5 * time.Minute)
	rechargeResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":         "ORD-RISK-HIT-001",
		"externalOrderNo": "EXT-RISK-HIT-001",
		"playerID":        player.ID,
		"gameID":          game.ID,
		"amount":          100.0,
		"currency":        "CNY",
		"status":          "paid",
		"paidAt":          paidAt.Format(time.RFC3339),
		"callbackAt":      callbackAt.Format(time.RFC3339),
		"channel":         "wechat",
		"idempotencyKey":  "recharge-risk-hit-1",
	})
	require.Equal(t, http.StatusCreated, rechargeResp.Code, rechargeResp.Body.String())

	var result struct {
		Order       model.RechargeOrder `json:"order"`
		Commissions int                 `json:"commissions"`
		Ledgers     int                 `json:"ledgers"`
		Duplicate   bool                `json:"duplicate"`
		Frozen      bool                `json:"frozen"`
	}
	require.NoError(t, json.Unmarshal(rechargeResp.Body.Bytes(), &result))
	require.True(t, result.Frozen)
	require.False(t, result.Duplicate)
	require.Equal(t, 0, result.Commissions)
	require.Equal(t, 0, result.Ledgers)
	require.Equal(t, "RISK-HIT-001", result.Order.RiskFlag)

	var storedOrder model.RechargeOrder
	require.NoError(t, testDB.First(&storedOrder, result.Order.ID).Error)
	require.Equal(t, "RISK-HIT-001", storedOrder.RiskFlag)

	var ledgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 1)
	require.Equal(t, model.LedgerTypeFreeze, ledgers[0].LedgerType)
	require.Equal(t, "risk_case", ledgers[0].ReferenceType)
	require.Equal(t, "RISK-HIT-001", ledgers[0].ReferenceID)

	var records []model.CommissionRecord
	require.NoError(t, testDB.Where("recharge_order_id = ?", result.Order.ID).Find(&records).Error)
	require.Len(t, records, 0)

	var refreshedAccount model.AgentAccount
	require.NoError(t, testDB.First(&refreshedAccount, account.ID).Error)
	require.InDelta(t, 200.0, refreshedAccount.Balance, 0.001)
	require.InDelta(t, 50.0, refreshedAccount.FrozenBalance, 0.001)
	require.InDelta(t, 150.0, refreshedAccount.WithdrawableAmount, 0.001)
}

func TestRechargeCallbackRequiresAgentGameAccessAndUsesPaidSettlementBase(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-ACCESS-BASE", "Access Base Agent")
	game := createGameFixture(t, testDB, "GAME-ACCESS-BASE", true)
	paidAt := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	player := model.Player{PlayerNo: "PLY-ACCESS-BASE", PlatformUserID: "platform-access-base", Currency: "CNY", Status: "active", RegisteredAt: paidAt.Add(-time.Hour)}
	require.NoError(t, testDB.Create(&player).Error)
	binding := model.Binding{PlayerID: player.ID, AgentID: agent.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: paidAt.Add(-time.Hour), EffectiveFrom: paidAt.Add(-time.Hour)}
	require.NoError(t, testDB.Create(&binding).Error)

	rule := model.CommissionRule{RuleName: "paid settlement base", Scope: model.RuleScopeAgentGame, RuleType: model.RuleTypeRatio, Status: model.RuleStatusPublished, Priority: 10, Version: 1, AgentID: &agent.ID, GameID: &game.ID, MaxSettlementDepth: 1, SettlementRate: 0.8, CommissionRate: 0.5, Currency: "CNY", EffectiveFrom: paidAt.Add(-time.Hour), PublishedAt: ptrTime(paidAt.Add(-30 * time.Minute)), PublishedBy: "admin", UniqueKey: "agent_game|paid-base"}
	require.NoError(t, testDB.Create(&rule).Error)
	rulePayload, err := json.Marshal(rule)
	require.NoError(t, err)
	require.NoError(t, testDB.Create(&model.RuleSnapshot{RuleID: rule.ID, RuleVersion: rule.Version, RuleUniqueKey: rule.UniqueKey, Scope: rule.Scope, AgentID: rule.AgentID, GameID: rule.GameID, SnapshotHash: "snapshot-paid-base", SnapshotPayload: rulePayload, EffectiveFrom: rule.EffectiveFrom, PublishedAt: *rule.PublishedAt, PublishedBy: "admin"}).Error)

	denied := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{"orderNo": "ORD-NO-ACCESS", "playerID": player.ID, "gameID": game.ID, "amount": 100.0, "currency": "CNY", "status": "paid", "paidAt": paidAt.Format(time.RFC3339), "idempotencyKey": "no-access"})
	require.Equal(t, http.StatusBadRequest, denied.Code, denied.Body.String())
	assertJSONErrorBody(t, denied, "agent does not have access")

	require.NoError(t, testDB.Create(&model.AgentGameAccess{AgentID: agent.ID, GameID: game.ID, Status: model.AccessStatusEnabled, GrantedBy: "admin", GrantedAt: paidAt.Add(-time.Hour), EffectiveFrom: paidAt.Add(-time.Hour)}).Error)
	accepted := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{"orderNo": "ORD-WITH-ACCESS", "playerID": player.ID, "gameID": game.ID, "amount": 100.0, "currency": "CNY", "status": "paid", "paidAt": paidAt.Format(time.RFC3339), "idempotencyKey": "with-access"})
	require.Equal(t, http.StatusCreated, accepted.Code, accepted.Body.String())

	var record model.CommissionRecord
	require.NoError(t, testDB.Where("record_no = ?", "COM-ORD-WITH-ACCESS-1").First(&record).Error)
	require.InDelta(t, 80.0, record.CommissionBaseAmount, 0.001)
	require.InDelta(t, 0.8, record.SettlementRate, 0.001)
	require.InDelta(t, 40.0, record.CommissionAmount, 0.001)
	require.NotNil(t, record.SourceAgentID)
	require.Equal(t, agent.ID, *record.SourceAgentID)
	require.NotEmpty(t, record.RelationSnapshot)

	var order model.RechargeOrder
	require.NoError(t, testDB.Where("order_no = ?", "ORD-WITH-ACCESS").First(&order).Error)
	require.InDelta(t, 100.0, order.PaidAmount, 0.001)

	var account model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).First(&account).Error)
	require.InDelta(t, 40.0, account.Balance, 0.001)
	require.InDelta(t, 40.0, account.AvailableBalance, 0.001)
	require.InDelta(t, 40.0, account.TotalIncome, 0.001)
}

func TestRechargeCallbackMetricsPreferActualProfitFacts(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-PROFIT-ACTUAL", "Profit Actual Agent")
	game := createGameFixture(t, testDB, "GAME-PROFIT-ACTUAL", true)
	paidAt := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	player := model.Player{PlayerNo: "PLY-PROFIT-ACTUAL", PlatformUserID: "platform-profit-actual", Currency: "CNY", Status: "active", RegisteredAt: paidAt.Add(-time.Hour)}
	require.NoError(t, testDB.Create(&player).Error)
	binding := model.Binding{PlayerID: player.ID, AgentID: agent.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: paidAt.Add(-time.Hour), EffectiveFrom: paidAt.Add(-time.Hour)}
	require.NoError(t, testDB.Create(&binding).Error)
	require.NoError(t, testDB.Create(&model.AgentGameAccess{AgentID: agent.ID, GameID: game.ID, Status: model.AccessStatusEnabled, GrantedBy: "admin", GrantedAt: paidAt.Add(-time.Hour), EffectiveFrom: paidAt.Add(-time.Hour)}).Error)

	rule := model.CommissionRule{RuleName: "actual profit metrics", Scope: model.RuleScopeAgentGame, RuleType: model.RuleTypeRatio, Status: model.RuleStatusPublished, Priority: 10, Version: 1, AgentID: &agent.ID, GameID: &game.ID, MaxSettlementDepth: 1, SettlementRate: 1, CommissionRate: 0.4, Currency: "CNY", EffectiveFrom: paidAt.Add(-time.Hour), PublishedAt: ptrTime(paidAt.Add(-30 * time.Minute)), PublishedBy: "admin", UniqueKey: "agent_game|actual-profit"}
	require.NoError(t, testDB.Create(&rule).Error)
	rulePayload, err := json.Marshal(rule)
	require.NoError(t, err)
	require.NoError(t, testDB.Create(&model.RuleSnapshot{RuleID: rule.ID, RuleVersion: rule.Version, RuleUniqueKey: rule.UniqueKey, Scope: rule.Scope, AgentID: rule.AgentID, GameID: rule.GameID, SnapshotHash: "snapshot-actual-profit", SnapshotPayload: rulePayload, EffectiveFrom: rule.EffectiveFrom, PublishedAt: *rule.PublishedAt, PublishedBy: "admin"}).Error)

	resp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":            "ORD-ACTUAL-PROFIT",
		"playerID":           player.ID,
		"gameID":             game.ID,
		"amount":             100.0,
		"paidAmount":         100.0,
		"paymentChannelCost": 2.0,
		"grossProfitAmount":  80.0,
		"currency":           "CNY",
		"status":             "paid",
		"paidAt":             paidAt.Format(time.RFC3339),
		"idempotencyKey":     "actual-profit-order",
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	metricsResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/data-platform-metrics", nil)
	require.Equal(t, http.StatusOK, metricsResp.Code, metricsResp.Body.String())
	var payload struct {
		Items []dataPlatformMetricItem `json:"items"`
		Total int                      `json:"total"`
	}
	require.NoError(t, json.Unmarshal(metricsResp.Body.Bytes(), &payload))
	metricByKey := make(map[string]dataPlatformMetricItem)
	for _, item := range payload.Items {
		metricByKey[item.Key] = item
	}
	require.InDelta(t, 100.0, metricByKey["profitDataCoverage"].Value, 0.001)
	require.InDelta(t, 2.0, metricByKey["paymentChannelCost"].Value, 0.001)
	require.InDelta(t, 80.0, metricByKey["estimatedGrossProfit"].Value, 0.001)
	require.InDelta(t, 78.0, metricByKey["estimatedNetProfit"].Value, 0.001)
	require.InDelta(t, 78.0, metricByKey["estimatedNetMargin"].Value, 0.001)
}

func TestOrderProfitFactsUpdateEndpoint(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-PROFIT-EDIT", "Profit Edit Agent")
	game := createGameFixture(t, testDB, "GAME-PROFIT-EDIT", true)
	paidAt := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
	player := model.Player{PlayerNo: "PLY-PROFIT-EDIT", PlatformUserID: "platform-profit-edit", Currency: "CNY", Status: "active", RegisteredAt: paidAt.Add(-time.Hour)}
	require.NoError(t, testDB.Create(&player).Error)
	binding := model.Binding{PlayerID: player.ID, AgentID: agent.ID, Status: model.BindingStatusBound, Source: model.BindingSourceRegister, BoundAt: paidAt.Add(-time.Hour), EffectiveFrom: paidAt.Add(-time.Hour)}
	require.NoError(t, testDB.Create(&binding).Error)
	require.NoError(t, testDB.Create(&model.AgentGameAccess{AgentID: agent.ID, GameID: game.ID, Status: model.AccessStatusEnabled, GrantedBy: "admin", GrantedAt: paidAt.Add(-time.Hour), EffectiveFrom: paidAt.Add(-time.Hour)}).Error)

	resp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":        "ORD-PROFIT-EDIT",
		"playerID":       player.ID,
		"gameID":         game.ID,
		"amount":         100.0,
		"currency":       "CNY",
		"status":         "paid",
		"paidAt":         paidAt.Format(time.RFC3339),
		"idempotencyKey": "profit-edit-order",
	})
	require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())

	var order model.RechargeOrder
	require.NoError(t, testDB.Where("order_no = ?", "ORD-PROFIT-EDIT").First(&order).Error)

	updateResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/orders/"+jsonUint(order.ID)+"/profit-facts", map[string]any{
		"paidAmount":         98.0,
		"paymentChannelCost": 1.5,
		"grossProfitAmount":  70.0,
		"remark":             "manual backfill",
	})
	require.Equal(t, http.StatusOK, updateResp.Code, updateResp.Body.String())

	require.NoError(t, testDB.First(&order, order.ID).Error)
	require.InDelta(t, 98.0, order.PaidAmount, 0.001)
	require.NotNil(t, order.PaymentChannelCost)
	require.InDelta(t, 1.5, *order.PaymentChannelCost, 0.001)
	require.NotNil(t, order.GrossProfitAmount)
	require.InDelta(t, 70.0, *order.GrossProfitAmount, 0.001)
	require.Contains(t, order.Remark, "manual backfill")
}

func TestRechargeCallbackRejectsInvalidDictionaryValues(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	game := createGameFixture(t, testDB, "GAME-ENUM-VALIDATE", true)

	resp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":        "ENUM-ORDER-INVALID",
		"playerID":       1,
		"gameID":         game.ID,
		"amount":         100,
		"currency":       "CNY",
		"status":         "paid",
		"rechargeType":   "illegal-type",
		"activityTags":   []string{"campaign-a"},
		"paidAt":         time.Now().UTC().Format(time.RFC3339),
		"idempotencyKey": "enum-order-invalid",
	})
	require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())
	assertJSONErrorBody(t, resp, "invalid rechargeType: illegal-type")

	validResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":        "ENUM-ORDER-INVALID-TAG",
		"playerID":       1,
		"gameID":         game.ID,
		"amount":         100,
		"currency":       "CNY",
		"status":         "paid",
		"rechargeType":   "normal",
		"activityTags":   []string{"illegal-tag"},
		"paidAt":         time.Now().UTC().Format(time.RFC3339),
		"idempotencyKey": "enum-order-invalid-tag",
	})
	require.Equal(t, http.StatusBadRequest, validResp.Code, validResp.Body.String())
	assertJSONErrorBody(t, validResp, "invalid activityTags: illegal-tag")
}

func TestRechargeCallbackRefundReversesCommissionAndLedger(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	parent := createAgentFixture(t, testDB, "AG-REF-PARENT", "Refund Parent")
	agent := createAgentFixture(t, testDB, "AG-REF-CHILD", "Refund Child")
	account := createAgentAccountFixture(t, testDB, agent.ID, "ACC-REF-CHILD")
	invite := createInviteCodeFixture(t, testDB, parent.ID, "INV-REF-PARENT")
	application := applyAndApproveAgentInvite(t, router, agent.ID, invite.Code)
	require.NotNil(t, application.ApprovedRelationID)

	game := createGameFixture(t, testDB, "GAME-REFUND", true)
	grantedAt := time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	require.NoError(t, testDB.Create(&model.AgentGameAccess{
		AgentID:       agent.ID,
		GameID:        game.ID,
		Status:        model.AccessStatusEnabled,
		GrantedBy:     "admin",
		GrantedAt:     grantedAt,
		EffectiveFrom: grantedAt,
	}).Error)

	player := model.Player{
		PlayerNo:       "PLY-REFUND",
		PlatformUserID: "platform-refund",
		Nickname:       "refund-player",
		Currency:       "CNY",
		Status:         "active",
		RegisteredAt:   grantedAt,
	}
	require.NoError(t, testDB.Create(&player).Error)
	require.NoError(t, testDB.Create(&model.Binding{
		PlayerID:      player.ID,
		AgentID:       agent.ID,
		InviteCodeID:  &invite.ID,
		Status:        model.BindingStatusBound,
		Source:        model.BindingSourceRegister,
		BoundAt:       grantedAt,
		EffectiveFrom: grantedAt,
		ApprovedBy:    "admin",
	}).Error)

	rule := model.CommissionRule{
		RuleName:           "refund rule",
		Scope:              model.RuleScopeAgentGame,
		RuleType:           model.RuleTypeRatio,
		Status:             model.RuleStatusPublished,
		Priority:           100,
		Version:            1,
		AgentID:            &agent.ID,
		GameID:             &game.ID,
		MaxSettlementDepth: 1,
		CommissionRate:     0.1,
		FixedAmount:        0,
		Currency:           "CNY",
		EffectiveFrom:      grantedAt.Add(-time.Hour),
		PublishedAt:        ptrTime(grantedAt),
		PublishedBy:        "admin",
		UniqueKey:          "agent_game|refund",
	}
	require.NoError(t, testDB.Create(&rule).Error)
	rulePayload, err := json.Marshal(rule)
	require.NoError(t, err)
	snapshot := model.RuleSnapshot{
		RuleID:          rule.ID,
		RuleVersion:     rule.Version,
		RuleUniqueKey:   rule.UniqueKey,
		Scope:           rule.Scope,
		AgentID:         rule.AgentID,
		GameID:          rule.GameID,
		SnapshotHash:    "snapshot-refund-1",
		SnapshotPayload: rulePayload,
		EffectiveFrom:   rule.EffectiveFrom,
		PublishedAt:     grantedAt,
		PublishedBy:     "admin",
	}
	require.NoError(t, testDB.Create(&snapshot).Error)

	paidAt := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	callbackAt := paidAt.Add(5 * time.Minute)
	orderNo := "ORD-REF-001"

	paidResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":         orderNo,
		"externalOrderNo": "EXT-REF-001",
		"playerID":        player.ID,
		"gameID":          game.ID,
		"amount":          100.0,
		"currency":        "CNY",
		"status":          "paid",
		"paidAt":          paidAt.Format(time.RFC3339),
		"callbackAt":      callbackAt.Format(time.RFC3339),
		"channel":         "wechat",
		"idempotencyKey":  "recharge-paid-refund-1",
	})
	require.Equal(t, http.StatusCreated, paidResp.Code, paidResp.Body.String())

	var paidResult struct {
		Order       model.RechargeOrder `json:"order"`
		Commissions int                 `json:"commissions"`
		Ledgers     int                 `json:"ledgers"`
		Duplicate   bool                `json:"duplicate"`
	}
	require.NoError(t, json.Unmarshal(paidResp.Body.Bytes(), &paidResult))
	require.Equal(t, 1, paidResult.Commissions)
	require.Equal(t, 1, paidResult.Ledgers)
	require.False(t, paidResult.Duplicate)

	var settledAccount model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", parent.ID).First(&settledAccount).Error)
	require.InDelta(t, 10.0, settledAccount.Balance, 0.001)
	require.InDelta(t, 10.0, settledAccount.WithdrawableAmount, 0.001)

	refundAt := callbackAt.Add(30 * time.Minute)
	refundResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":         orderNo,
		"externalOrderNo": "EXT-REF-001-R",
		"playerID":        player.ID,
		"gameID":          game.ID,
		"amount":          100.0,
		"currency":        "CNY",
		"status":          "refunded",
		"paidAt":          paidAt.Format(time.RFC3339),
		"callbackAt":      refundAt.Format(time.RFC3339),
		"channel":         "wechat",
		"idempotencyKey":  "recharge-refund-refund-1",
	})
	require.Equal(t, http.StatusCreated, refundResp.Code, refundResp.Body.String())

	var refundResult struct {
		Order       model.RechargeOrder `json:"order"`
		Commissions int                 `json:"commissions"`
		Ledgers     int                 `json:"ledgers"`
		Duplicate   bool                `json:"duplicate"`
	}
	require.NoError(t, json.Unmarshal(refundResp.Body.Bytes(), &refundResult))
	require.Equal(t, model.OrderStatusRefunded, refundResult.Order.Status)
	require.Equal(t, 1, refundResult.Commissions)
	require.Equal(t, 1, refundResult.Ledgers)
	require.False(t, refundResult.Duplicate)

	var records []model.CommissionRecord
	require.NoError(t, testDB.Where("recharge_order_id = ?", refundResult.Order.ID).Order("id asc").Find(&records).Error)
	require.Len(t, records, 2)
	require.Nil(t, records[0].ReversedFromID)
	require.Equal(t, model.CommissionStatusReversed, records[0].Status)
	require.InDelta(t, 10.0, records[0].CommissionAmount, 0.001)
	require.NotNil(t, records[1].ReversedFromID)
	require.Equal(t, records[0].ID, *records[1].ReversedFromID)
	require.Equal(t, model.CommissionStatusReversed, records[1].Status)
	require.InDelta(t, -10.0, records[1].CommissionAmount, 0.001)

	var ledgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", parent.ID).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 2)
	require.Equal(t, model.LedgerTypeIncome, ledgers[0].LedgerType)
	require.Equal(t, model.LedgerTypeReverse, ledgers[1].LedgerType)
	require.Equal(t, model.LedgerDirectionDebit, ledgers[1].Direction)
	require.InDelta(t, 10.0, ledgers[1].Amount, 0.001)
	require.InDelta(t, 10.0, ledgers[1].BalanceBefore, 0.001)
	require.InDelta(t, 0.0, ledgers[1].BalanceAfter, 0.001)

	var refundedAccount model.AgentAccount
	require.NoError(t, testDB.First(&refundedAccount, account.ID).Error)
	require.InDelta(t, 0.0, refundedAccount.Balance, 0.001)
	require.InDelta(t, 0.0, refundedAccount.WithdrawableAmount, 0.001)

	duplicateRefundResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/recharge/callback", map[string]any{
		"orderNo":         orderNo,
		"externalOrderNo": "EXT-REF-001-R2",
		"playerID":        player.ID,
		"gameID":          game.ID,
		"amount":          100.0,
		"currency":        "CNY",
		"status":          "refunded",
		"paidAt":          paidAt.Format(time.RFC3339),
		"callbackAt":      refundAt.Add(time.Minute).Format(time.RFC3339),
		"channel":         "wechat",
		"idempotencyKey":  "recharge-refund-refund-2",
	})
	require.Equal(t, http.StatusOK, duplicateRefundResp.Code, duplicateRefundResp.Body.String())

	var duplicateResult struct {
		Order       model.RechargeOrder `json:"order"`
		Commissions int                 `json:"commissions"`
		Ledgers     int                 `json:"ledgers"`
		Duplicate   bool                `json:"duplicate"`
	}
	require.NoError(t, json.Unmarshal(duplicateRefundResp.Body.Bytes(), &duplicateResult))
	require.True(t, duplicateResult.Duplicate)
	require.Equal(t, 0, duplicateResult.Commissions)
	require.Equal(t, 0, duplicateResult.Ledgers)

	var finalRecords []model.CommissionRecord
	require.NoError(t, testDB.Where("recharge_order_id = ?", refundResult.Order.ID).Find(&finalRecords).Error)
	require.Len(t, finalRecords, 2)
	var finalLedgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", parent.ID).Find(&finalLedgers).Error)
	require.Len(t, finalLedgers, 2)

	var callbackLogs []model.RechargeCallbackLog
	require.NoError(t, testDB.Where("recharge_order_id = ?", refundResult.Order.ID).Find(&callbackLogs).Error)
	require.Len(t, callbackLogs, 3)
	var finalAccount model.AgentAccount
	require.NoError(t, testDB.First(&finalAccount, account.ID).Error)
	require.InDelta(t, 0.0, finalAccount.Balance, 0.001)
	require.InDelta(t, 0.0, finalAccount.WithdrawableAmount, 0.001)
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func TestLedgerOrderNoFilter(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-LEDGER-FILTER", "Ledger Filter Agent")
	account := createAgentAccountFixture(t, testDB, agent.ID, "ACC-LEDGER-FILTER")
	game := createGameFixture(t, testDB, "GAME-LEDGER-FILTER", true)

	agentID := agent.ID
	matchedOrder := model.RechargeOrder{
		OrderNo:         "ORDER-LEDGER-MATCH",
		ExternalOrderNo: "EXT-LEDGER-MATCH",
		PlayerID:        3001,
		GameID:          game.ID,
		AgentID:         &agentID,
		Amount:          50,
		Currency:        "CNY",
		Status:          model.OrderStatusPaid,
		IdempotencyKey:  "order-ledger-match",
	}
	unmatchedOrder := model.RechargeOrder{
		OrderNo:         "ORDER-LEDGER-OTHER",
		ExternalOrderNo: "EXT-LEDGER-OTHER",
		PlayerID:        3002,
		GameID:          game.ID,
		AgentID:         &agentID,
		Amount:          25,
		Currency:        "CNY",
		Status:          model.OrderStatusPaid,
		IdempotencyKey:  "order-ledger-other",
	}
	require.NoError(t, testDB.Create(&matchedOrder).Error)
	require.NoError(t, testDB.Create(&unmatchedOrder).Error)

	matchedCommission := model.CommissionRecord{
		RecordNo:             "COMM-LEDGER-MATCH",
		RechargeOrderID:      matchedOrder.ID,
		PlayerID:             matchedOrder.PlayerID,
		AgentID:              agent.ID,
		GameID:               game.ID,
		SettlementDepth:      1,
		CommissionBaseAmount: matchedOrder.Amount,
		CommissionRate:       0.1,
		CommissionAmount:     5,
		Currency:             "CNY",
		Status:               model.CommissionStatusPending,
		EstimatedAt:          time.Now().UTC(),
	}
	unmatchedCommission := model.CommissionRecord{
		RecordNo:             "COMM-LEDGER-OTHER",
		RechargeOrderID:      unmatchedOrder.ID,
		PlayerID:             unmatchedOrder.PlayerID,
		AgentID:              agent.ID,
		GameID:               game.ID,
		SettlementDepth:      1,
		CommissionBaseAmount: unmatchedOrder.Amount,
		CommissionRate:       0.1,
		CommissionAmount:     2.5,
		Currency:             "CNY",
		Status:               model.CommissionStatusPending,
		EstimatedAt:          time.Now().UTC(),
	}
	require.NoError(t, testDB.Create(&matchedCommission).Error)
	require.NoError(t, testDB.Create(&unmatchedCommission).Error)

	matchedLedger := model.AgentAccountLedger{
		AccountID:      account.ID,
		AgentID:        agent.ID,
		ReferenceType:  "commission_record",
		ReferenceID:    strconv.FormatUint(matchedCommission.ID, 10),
		LedgerType:     model.LedgerTypeIncome,
		Direction:      model.LedgerDirectionCredit,
		Amount:         5,
		BalanceBefore:  0,
		BalanceAfter:   5,
		FrozenBefore:   0,
		FrozenAfter:    0,
		Currency:       "CNY",
		OccurredAt:     time.Now().UTC(),
		IdempotencyKey: "ledger-match",
	}
	unmatchedLedger := model.AgentAccountLedger{
		AccountID:      account.ID,
		AgentID:        agent.ID,
		ReferenceType:  "commission_record",
		ReferenceID:    strconv.FormatUint(unmatchedCommission.ID, 10),
		LedgerType:     model.LedgerTypeIncome,
		Direction:      model.LedgerDirectionCredit,
		Amount:         2.5,
		BalanceBefore:  5,
		BalanceAfter:   7.5,
		FrozenBefore:   0,
		FrozenAfter:    0,
		Currency:       "CNY",
		OccurredAt:     time.Now().UTC(),
		IdempotencyKey: "ledger-other",
	}
	require.NoError(t, testDB.Create(&matchedLedger).Error)
	require.NoError(t, testDB.Create(&unmatchedLedger).Error)

	resp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/ledger?orderNo=ORDER-LEDGER-MATCH", nil)
	require.Equal(t, http.StatusOK, resp.Code)

	var ledgerList struct {
		Items []model.AgentAccountLedger `json:"items"`
		Total int                        `json:"total"`
	}
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &ledgerList))
	require.Equal(t, 1, ledgerList.Total)
	require.Len(t, ledgerList.Items, 1)
	require.Equal(t, matchedLedger.ID, ledgerList.Items[0].ID)
	require.Equal(t, strconv.FormatUint(matchedCommission.ID, 10), ledgerList.Items[0].ReferenceID)
}

func TestAgentAccountRiskReportAndReversalFlow(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	seedBindingFixtures(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-RISK", "Risk Agent")
	account := createAgentAccountFixture(t, testDB, agent.ID, "ACC-RISK")

	createRiskCaseResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
		"caseNo":   "RISK-001",
		"agentID":  agent.ID,
		"reason":   "suspicious activity",
		"freeze":   true,
		"amount":   30.25,
		"currency": "CNY",
		"remark":   "freeze for review",
	})
	require.Equal(t, http.StatusBadRequest, createRiskCaseResp.Code)

	income := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        agent.ID,
		"referenceType":  "commission_record",
		"referenceID":    "COM-RISK-1",
		"ledgerType":     "income",
		"amount":         120.5,
		"currency":       "CNY",
		"idempotencyKey": "ledger-income-risk",
		"remark":         "income funding",
	})
	require.Equal(t, http.StatusCreated, income.Code, income.Body.String())
	var incomeLedger model.AgentAccountLedger
	require.NoError(t, json.Unmarshal(income.Body.Bytes(), &incomeLedger))
	require.Equal(t, model.LedgerTypeIncome, incomeLedger.LedgerType)

	createRiskCaseResp = performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
		"caseNo":   "RISK-001",
		"agentID":  agent.ID,
		"reason":   "suspicious activity",
		"freeze":   true,
		"amount":   30.25,
		"currency": "CNY",
		"remark":   "freeze for review",
	})
	require.Equal(t, http.StatusCreated, createRiskCaseResp.Code, createRiskCaseResp.Body.String())
	var createdRiskCase struct {
		CaseNo       string                    `json:"caseNo"`
		Frozen       bool                      `json:"frozen"`
		FrozenLedger *model.AgentAccountLedger `json:"frozenLedger"`
	}
	require.NoError(t, json.Unmarshal(createRiskCaseResp.Body.Bytes(), &createdRiskCase))
	require.Equal(t, "RISK-001", createdRiskCase.CaseNo)
	require.True(t, createdRiskCase.Frozen)
	require.NotNil(t, createdRiskCase.FrozenLedger)
	freezeLedger := *createdRiskCase.FrozenLedger
	require.Equal(t, model.LedgerTypeFreeze, freezeLedger.LedgerType)
	require.InDelta(t, 120.5, freezeLedger.BalanceAfter, 0.001)
	require.InDelta(t, 30.25, freezeLedger.FrozenAfter, 0.001)

	withdrawalCreate := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         agent.ID,
		"amount":          15.5,
		"currency":        "CNY",
		"channel":         "bank_transfer",
		"bankAccountName": "Risk Agent",
		"bankAccountNo":   "6222000000001",
		"bankName":        "Test Bank",
		"idempotencyKey":  "withdrawal-risk-1",
		"remark":          "phase3 withdrawal",
	})
	require.Equal(t, http.StatusCreated, withdrawalCreate.Code, withdrawalCreate.Body.String())
	var createdWithdrawal model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(withdrawalCreate.Body.Bytes(), &createdWithdrawal))
	require.Equal(t, model.WithdrawalStatusPending, createdWithdrawal.Status)
	require.Equal(t, "t1", createdWithdrawal.TenantCode)
	require.NotNil(t, createdWithdrawal.ApprovedLedgerID)

	listWithdrawalsResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/withdrawals?tenantCode=t1&status=pending&agentID="+jsonUint(agent.ID), nil)
	require.Equal(t, http.StatusOK, listWithdrawalsResp.Code)
	var withdrawalList struct {
		Items []struct {
			model.WithdrawalRequest
			AgentName string `json:"agentName"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listWithdrawalsResp.Body.Bytes(), &withdrawalList))
	require.Equal(t, 1, withdrawalList.Total)
	require.Equal(t, agent.Name, withdrawalList.Items[0].AgentName)

	requestNoListResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/withdrawals?requestNo="+createdWithdrawal.RequestNo, nil)
	require.Equal(t, http.StatusOK, requestNoListResp.Code)
	var requestNoList struct {
		Items []struct {
			model.WithdrawalRequest
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(requestNoListResp.Body.Bytes(), &requestNoList))
	require.Equal(t, 1, requestNoList.Total)
	require.Equal(t, createdWithdrawal.ID, requestNoList.Items[0].ID)

	missingRequestNoResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/withdrawals?requestNo=WD-NOT-FOUND", nil)
	require.Equal(t, http.StatusOK, missingRequestNoResp.Code)
	require.NoError(t, json.Unmarshal(missingRequestNoResp.Body.Bytes(), &requestNoList))
	require.Zero(t, requestNoList.Total)

	approveWithdrawal := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(createdWithdrawal.ID)+"/review", map[string]any{
		"action": "approve",
		"remark": "paid out",
	})
	require.Equal(t, http.StatusOK, approveWithdrawal.Code, approveWithdrawal.Body.String())
	var approvedWithdrawal model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(approveWithdrawal.Body.Bytes(), &approvedWithdrawal))
	require.Equal(t, model.WithdrawalStatusApproved, approvedWithdrawal.Status)
	require.Nil(t, approvedWithdrawal.CompletedLedgerID)
	require.Nil(t, approvedWithdrawal.PaidAt)

	payoutWithdrawal := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(createdWithdrawal.ID)+"/payout", map[string]any{
		"action":    "success",
		"remark":    "finance settled",
		"reference": "PO-001",
	})
	require.Equal(t, http.StatusOK, payoutWithdrawal.Code, payoutWithdrawal.Body.String())
	var paidWithdrawal model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(payoutWithdrawal.Body.Bytes(), &paidWithdrawal))
	require.Equal(t, model.WithdrawalStatusPaid, paidWithdrawal.Status)
	require.NotNil(t, paidWithdrawal.CompletedLedgerID)
	require.NotNil(t, paidWithdrawal.PaidAt)
	require.Equal(t, "PO-001", paidWithdrawal.PayoutReference)

	require.NoError(t, testDB.First(&account, account.ID).Error)
	require.InDelta(t, 105.0, account.Balance, 0.001)
	require.InDelta(t, 0.0, account.FrozenBalance, 0.001)
	require.InDelta(t, 105.0, account.WithdrawableAmount, 0.001)

	require.NoError(t, testDB.First(&createdWithdrawal, createdWithdrawal.ID).Error)
	require.Equal(t, model.WithdrawalStatusPaid, createdWithdrawal.Status)
	require.NotNil(t, createdWithdrawal.CompletedLedgerID)
	require.NotNil(t, createdWithdrawal.PaidAt)

	releaseRiskCase := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/RISK-001/review", map[string]any{
		"action": "release",
		"remark": "partial release",
	})
	require.Equal(t, http.StatusBadRequest, releaseRiskCase.Code)
	require.Contains(t, releaseRiskCase.Body.String(), "risk case already reviewed")

	reversal := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        agent.ID,
		"referenceType":  "refund_order",
		"referenceID":    "REFUND-001",
		"ledgerType":     "reverse",
		"amount":         20.0,
		"currency":       "CNY",
		"idempotencyKey": "ledger-reverse-risk",
		"remark":         "refund reversal",
	})
	require.Equal(t, http.StatusCreated, reversal.Code, reversal.Body.String())

	listLedgerResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/agent-accounts/ledger?agentID="+jsonUint(agent.ID), nil)
	require.Equal(t, http.StatusOK, listLedgerResp.Code)
	var ledgerList struct {
		Items []model.AgentAccountLedger `json:"items"`
		Total int                        `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listLedgerResp.Body.Bytes(), &ledgerList))
	require.Equal(t, 7, ledgerList.Total)

	require.NoError(t, testDB.First(&account, account.ID).Error)
	require.InDelta(t, 105.0, account.Balance, 0.001)
	require.InDelta(t, 0.0, account.FrozenBalance, 0.001)
	require.InDelta(t, 105.0, account.WithdrawableAmount, 0.001)

	riskResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/risk/agent-accounts?minFrozenAmount=15&riskLevel=medium&agentID="+jsonUint(agent.ID), nil)
	require.Equal(t, http.StatusOK, riskResp.Code)
	var riskList struct {
		Items []struct {
			model.AgentAccount
			AgentName   string  `json:"agentName"`
			RiskLevel   string  `json:"riskLevel"`
			FrozenRatio float64 `json:"frozenRatio"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(riskResp.Body.Bytes(), &riskList))
	require.Equal(t, 1, riskList.Total)
	require.Equal(t, agent.Name, riskList.Items[0].AgentName)
	require.Equal(t, agent.ID, riskList.Items[0].AgentID)
	require.Equal(t, "medium", riskList.Items[0].RiskLevel)
	require.InDelta(t, 15.5/100.5, riskList.Items[0].FrozenRatio, 0.0001)

	agentReportResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/agent-performance?agentID="+jsonUint(agent.ID), nil)
	require.Equal(t, http.StatusOK, agentReportResp.Code)
	var agentReport struct {
		Items []struct {
			AgentID            uint64  `json:"agentID"`
			AgentName          string  `json:"agentName"`
			Currency           string  `json:"currency"`
			Balance            float64 `json:"balance"`
			FrozenBalance      float64 `json:"frozenBalance"`
			WithdrawableAmount float64 `json:"withdrawableAmount"`
			TotalEntries       int64   `json:"totalEntries"`
			IncomeAmount       float64 `json:"incomeAmount"`
			FreezeAmount       float64 `json:"freezeAmount"`
			UnfreezeAmount     float64 `json:"unfreezeAmount"`
			DebitAmount        float64 `json:"debitAmount"`
			ReverseAmount      float64 `json:"reverseAmount"`
			AdjustAmount       float64 `json:"adjustAmount"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(agentReportResp.Body.Bytes(), &agentReport))
	require.Equal(t, 1, agentReport.Total)
	require.Equal(t, agent.ID, agentReport.Items[0].AgentID)
	require.Equal(t, "CNY", agentReport.Items[0].Currency)
	require.EqualValues(t, 7, agentReport.Items[0].TotalEntries)
	require.InDelta(t, 120.5, agentReport.Items[0].IncomeAmount, 0.001)
	require.InDelta(t, 50.25, agentReport.Items[0].DebitAmount, 0.001)
	require.InDelta(t, 35.5, agentReport.Items[0].ReverseAmount, 0.001)
	require.InDelta(t, 45.75, agentReport.Items[0].FreezeAmount, 0.001)
	require.InDelta(t, 45.75, agentReport.Items[0].UnfreezeAmount, 0.001)
	require.InDelta(t, 0, agentReport.Items[0].AdjustAmount, 0.001)

	settlementResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/game-settlement?currency=CNY", nil)
	require.Equal(t, http.StatusOK, settlementResp.Code)
	var settlementReport struct {
		Items []struct {
			Currency              string  `json:"currency"`
			TotalBills            int64   `json:"totalBills"`
			ConfirmedBills        int64   `json:"confirmedBills"`
			PendingBills          int64   `json:"pendingBills"`
			GeneratedBills        int64   `json:"generatedBills"`
			CancelledBills        int64   `json:"cancelledBills"`
			TotalCommissionAmount float64 `json:"totalCommissionAmount"`
			TotalAdjustmentAmount float64 `json:"totalAdjustmentAmount"`
			TotalPayableAmount    float64 `json:"totalPayableAmount"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(settlementResp.Body.Bytes(), &settlementReport))
	require.GreaterOrEqual(t, settlementReport.Total, 1)
	var cnySettlement *struct {
		Currency              string  `json:"currency"`
		TotalBills            int64   `json:"totalBills"`
		ConfirmedBills        int64   `json:"confirmedBills"`
		PendingBills          int64   `json:"pendingBills"`
		GeneratedBills        int64   `json:"generatedBills"`
		CancelledBills        int64   `json:"cancelledBills"`
		TotalCommissionAmount float64 `json:"totalCommissionAmount"`
		TotalAdjustmentAmount float64 `json:"totalAdjustmentAmount"`
		TotalPayableAmount    float64 `json:"totalPayableAmount"`
	}
	for i := range settlementReport.Items {
		if settlementReport.Items[i].Currency == "CNY" {
			cnySettlement = &settlementReport.Items[i]
			break
		}
	}
	require.NotNil(t, cnySettlement)
	require.Equal(t, "CNY", cnySettlement.Currency)
	require.EqualValues(t, 0, cnySettlement.PendingBills)
	require.EqualValues(t, 0, cnySettlement.CancelledBills)
	require.InDelta(t, 0.0, cnySettlement.TotalCommissionAmount, 0.001)
	require.InDelta(t, 0.0, cnySettlement.TotalAdjustmentAmount, 0.001)
	require.InDelta(t, 0.0, cnySettlement.TotalPayableAmount, 0.001)

	teamReportResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/team-performance?agentID="+jsonUint(agent.ID)+"&currency=CNY", nil)
	require.Equal(t, http.StatusOK, teamReportResp.Code)
	var teamReport struct {
		Items []struct {
			AgentID                 uint64  `json:"agentID"`
			AgentName               string  `json:"agentName"`
			Level                   uint32  `json:"level"`
			Currency                string  `json:"currency"`
			DirectDescendants       int64   `json:"directDescendants"`
			TotalDescendants        int64   `json:"totalDescendants"`
			ActiveDescendants       int64   `json:"activeDescendants"`
			LeafDescendants         int64   `json:"leafDescendants"`
			BoundPlayers            int64   `json:"boundPlayers"`
			TotalTeamBalance        float64 `json:"totalTeamBalance"`
			TotalTeamFrozenBalance  float64 `json:"totalTeamFrozenBalance"`
			TotalWithdrawableAmount float64 `json:"totalWithdrawableAmount"`
			PendingWithdrawals      int64   `json:"pendingWithdrawals"`
			PendingWithdrawalAmount float64 `json:"pendingWithdrawalAmount"`
			ConfirmedBills          int64   `json:"confirmedBills"`
			ConfirmedBillAmount     float64 `json:"confirmedBillAmount"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(teamReportResp.Body.Bytes(), &teamReport))
	require.Equal(t, 1, teamReport.Total)
	require.Equal(t, agent.ID, teamReport.Items[0].AgentID)
	require.Equal(t, agent.Name, teamReport.Items[0].AgentName)
	require.Equal(t, uint32(1), teamReport.Items[0].Level)
	require.Equal(t, "CNY", teamReport.Items[0].Currency)
	require.EqualValues(t, 0, teamReport.Items[0].DirectDescendants)
	require.EqualValues(t, 0, teamReport.Items[0].TotalDescendants)
	require.EqualValues(t, 0, teamReport.Items[0].ActiveDescendants)
	require.EqualValues(t, 0, teamReport.Items[0].LeafDescendants)
	require.EqualValues(t, 0, teamReport.Items[0].BoundPlayers)
	require.InDelta(t, 100.5, teamReport.Items[0].TotalTeamBalance, 0.001)
	require.InDelta(t, 15.5, teamReport.Items[0].TotalTeamFrozenBalance, 0.001)
	require.InDelta(t, 85.0, teamReport.Items[0].TotalWithdrawableAmount, 0.001)
	require.EqualValues(t, 0, teamReport.Items[0].PendingWithdrawals)
	require.InDelta(t, 0.0, teamReport.Items[0].PendingWithdrawalAmount, 0.001)
	require.EqualValues(t, 0, teamReport.Items[0].ConfirmedBills)
	require.InDelta(t, 0.0, teamReport.Items[0].ConfirmedBillAmount, 0.001)

	generatedAt := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Second)
	confirmedAt := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, testDB.Create(&model.SettlementBill{
		BillNo:           "SB-REPORT-GEN",
		AgentID:          agent.ID,
		PeriodStart:      generatedAt.Add(-24 * time.Hour),
		PeriodEnd:        generatedAt,
		Currency:         "CNY",
		Status:           model.SettlementBillStatusGenerated,
		CommissionAmount: 20,
		AdjustmentAmount: 1.5,
		PayableAmount:    21.5,
		GeneratedAt:      &generatedAt,
	}).Error)
	require.NoError(t, testDB.Create(&model.SettlementBill{
		BillNo:           "SB-REPORT-CFM",
		AgentID:          agent.ID,
		PeriodStart:      confirmedAt.Add(-24 * time.Hour),
		PeriodEnd:        confirmedAt,
		Currency:         "CNY",
		Status:           model.SettlementBillStatusConfirmed,
		CommissionAmount: 30,
		AdjustmentAmount: 2,
		PayableAmount:    32,
		GeneratedAt:      &generatedAt,
		ConfirmedAt:      &confirmedAt,
		ConfirmedBy:      "finance",
	}).Error)
	require.NoError(t, testDB.Create(&model.SettlementBill{
		BillNo:           "SB-REPORT-CAN",
		AgentID:          agent.ID,
		PeriodStart:      generatedAt.Add(-48 * time.Hour),
		PeriodEnd:        generatedAt.Add(-24 * time.Hour),
		Currency:         "CNY",
		Status:           model.SettlementBillStatusCancelled,
		CommissionAmount: 10,
		AdjustmentAmount: 0,
		PayableAmount:    10,
		GeneratedAt:      &generatedAt,
	}).Error)

	progressResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/settlement-progress?currency=CNY", nil)
	require.Equal(t, http.StatusOK, progressResp.Code)
	var progressReport struct {
		Items []struct {
			Currency                string  `json:"currency"`
			TotalBills              int64   `json:"totalBills"`
			GeneratedBills          int64   `json:"generatedBills"`
			ConfirmedBills          int64   `json:"confirmedBills"`
			CancelledBills          int64   `json:"cancelledBills"`
			ProgressPercent         float64 `json:"progressPercent"`
			PendingCommissionAmount float64 `json:"pendingCommissionAmount"`
			PendingPayableAmount    float64 `json:"pendingPayableAmount"`
			CompletedPayableAmount  float64 `json:"completedPayableAmount"`
			LastGeneratedAt         string  `json:"lastGeneratedAt"`
			LastConfirmedAt         string  `json:"lastConfirmedAt"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(progressResp.Body.Bytes(), &progressReport))
	require.Equal(t, 1, progressReport.Total)
	require.Equal(t, "CNY", progressReport.Items[0].Currency)
	require.EqualValues(t, 3, progressReport.Items[0].TotalBills)
	require.EqualValues(t, 1, progressReport.Items[0].GeneratedBills)
	require.EqualValues(t, 1, progressReport.Items[0].ConfirmedBills)
	require.EqualValues(t, 1, progressReport.Items[0].CancelledBills)
	require.InDelta(t, 33.33, progressReport.Items[0].ProgressPercent, 0.01)
	require.InDelta(t, 20.0, progressReport.Items[0].PendingCommissionAmount, 0.001)
	require.InDelta(t, 21.5, progressReport.Items[0].PendingPayableAmount, 0.001)
	require.InDelta(t, 32.0, progressReport.Items[0].CompletedPayableAmount, 0.001)
	require.Equal(t, generatedAt.Format(time.RFC3339), progressReport.Items[0].LastGeneratedAt)
	require.Equal(t, confirmedAt.Format(time.RFC3339), progressReport.Items[0].LastConfirmedAt)
}

func TestReportEndpointsTenantIsolation(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenantTwo := model.Tenant{Code: "t2", Name: "Tenant Two", DisplayName: "Tenant Two", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantTwo).Error)
	brandTwo := model.Brand{TenantID: tenantTwo.ID, Code: "b2", Name: "Brand Two", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandTwo).Error)
	tenantOne := model.Tenant{Code: "t1", Name: "Tenant One", DisplayName: "Tenant One", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	brandOne := model.Brand{TenantID: tenantOne.ID, Code: "b1", Name: "Brand One", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&brandOne).Error)
	scopeTestUser(t, testDB, "finance", &tenantOne.ID, &brandOne.ID)

	inScopeAgent := createAgentFixture(t, testDB, "AG-REPORT-T1", "Scoped Report Agent")
	require.NoError(t, testDB.Model(&inScopeAgent).Updates(map[string]any{"tenant_id": tenantOne.ID, "brand_id": brandOne.ID, "currency": "CNY", "remark": "t1"}).Error)
	createAgentAccountFixture(t, testDB, inScopeAgent.ID, "ACC-REPORT-T1")

	outOfScopeAgent := createAgentFixture(t, testDB, "AG-REPORT-T2", "Other Tenant Agent")
	require.NoError(t, testDB.Model(&outOfScopeAgent).Updates(map[string]any{"tenant_id": tenantTwo.ID, "brand_id": brandTwo.ID, "currency": "CNY", "remark": "t2"}).Error)
	createAgentAccountFixture(t, testDB, outOfScopeAgent.ID, "ACC-REPORT-T2")

	generatedAt := time.Date(2026, 4, 20, 10, 0, 0, 0, time.UTC)
	confirmedAt := generatedAt.Add(2 * time.Hour)

	require.NoError(t, testDB.Create(&model.SettlementBill{
		BillNo:           "SB-ISO-T1-CFM",
		AgentID:          inScopeAgent.ID,
		PeriodStart:      generatedAt.Add(-24 * time.Hour),
		PeriodEnd:        generatedAt,
		Currency:         "CNY",
		Status:           model.SettlementBillStatusConfirmed,
		CommissionAmount: 11,
		AdjustmentAmount: 1,
		PayableAmount:    12,
		GeneratedAt:      &generatedAt,
		ConfirmedAt:      &confirmedAt,
		ConfirmedBy:      "finance",
	}).Error)
	require.NoError(t, testDB.Create(&model.SettlementBill{
		BillNo:           "SB-ISO-T2-GEN",
		AgentID:          outOfScopeAgent.ID,
		PeriodStart:      generatedAt.Add(-48 * time.Hour),
		PeriodEnd:        generatedAt.Add(-24 * time.Hour),
		Currency:         "CNY",
		Status:           model.SettlementBillStatusGenerated,
		CommissionAmount: 70,
		AdjustmentAmount: 5,
		PayableAmount:    75,
		GeneratedAt:      &generatedAt,
	}).Error)

	gameSettlementResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/game-settlement?currency=CNY", nil)
	require.Equal(t, http.StatusOK, gameSettlementResp.Code)
	var gameSettlement struct {
		Items []struct {
			Currency              string  `json:"currency"`
			TotalBills            int64   `json:"totalBills"`
			ConfirmedBills        int64   `json:"confirmedBills"`
			PendingBills          int64   `json:"pendingBills"`
			GeneratedBills        int64   `json:"generatedBills"`
			CancelledBills        int64   `json:"cancelledBills"`
			TotalCommissionAmount float64 `json:"totalCommissionAmount"`
			TotalAdjustmentAmount float64 `json:"totalAdjustmentAmount"`
			TotalPayableAmount    float64 `json:"totalPayableAmount"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(gameSettlementResp.Body.Bytes(), &gameSettlement))
	require.Equal(t, 1, gameSettlement.Total)
	require.Equal(t, "CNY", gameSettlement.Items[0].Currency)
	require.EqualValues(t, 1, gameSettlement.Items[0].TotalBills)
	require.EqualValues(t, 1, gameSettlement.Items[0].ConfirmedBills)
	require.EqualValues(t, 0, gameSettlement.Items[0].GeneratedBills)
	require.InDelta(t, 11.0, gameSettlement.Items[0].TotalCommissionAmount, 0.001)
	require.InDelta(t, 1.0, gameSettlement.Items[0].TotalAdjustmentAmount, 0.001)
	require.InDelta(t, 12.0, gameSettlement.Items[0].TotalPayableAmount, 0.001)

	teamPerformanceResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/team-performance?currency=CNY", nil)
	require.Equal(t, http.StatusOK, teamPerformanceResp.Code)
	var teamPerformance struct {
		Items []struct {
			AgentID   uint64 `json:"agentID"`
			AgentName string `json:"agentName"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(teamPerformanceResp.Body.Bytes(), &teamPerformance))
	require.Equal(t, 1, teamPerformance.Total)
	require.Len(t, teamPerformance.Items, 1)
	require.Equal(t, inScopeAgent.ID, teamPerformance.Items[0].AgentID)
	require.Equal(t, inScopeAgent.Name, teamPerformance.Items[0].AgentName)

	progressResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/settlement-progress?currency=CNY", nil)
	require.Equal(t, http.StatusOK, progressResp.Code)
	var progressReport struct {
		Items []struct {
			Currency                string  `json:"currency"`
			TotalBills              int64   `json:"totalBills"`
			GeneratedBills          int64   `json:"generatedBills"`
			ConfirmedBills          int64   `json:"confirmedBills"`
			CancelledBills          int64   `json:"cancelledBills"`
			ProgressPercent         float64 `json:"progressPercent"`
			PendingCommissionAmount float64 `json:"pendingCommissionAmount"`
			PendingPayableAmount    float64 `json:"pendingPayableAmount"`
			CompletedPayableAmount  float64 `json:"completedPayableAmount"`
			LastGeneratedAt         string  `json:"lastGeneratedAt"`
			LastConfirmedAt         string  `json:"lastConfirmedAt"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(progressResp.Body.Bytes(), &progressReport))
	require.Equal(t, 1, progressReport.Total)
	require.Equal(t, "CNY", progressReport.Items[0].Currency)
	require.EqualValues(t, 1, progressReport.Items[0].TotalBills)
	require.EqualValues(t, 0, progressReport.Items[0].GeneratedBills)
	require.EqualValues(t, 1, progressReport.Items[0].ConfirmedBills)
	require.EqualValues(t, 0, progressReport.Items[0].CancelledBills)
	require.InDelta(t, 100.0, progressReport.Items[0].ProgressPercent, 0.001)
	require.InDelta(t, 0.0, progressReport.Items[0].PendingCommissionAmount, 0.001)
	require.InDelta(t, 0.0, progressReport.Items[0].PendingPayableAmount, 0.001)
	require.InDelta(t, 12.0, progressReport.Items[0].CompletedPayableAmount, 0.001)
	require.Empty(t, progressReport.Items[0].LastGeneratedAt)
	require.Equal(t, confirmedAt.Format(time.RFC3339), progressReport.Items[0].LastConfirmedAt)
}

func TestWithdrawalReviewRejectUnfreezesLedger(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-WD-REJECT", "Withdrawal Reject Agent")
	createAgentAccountFixture(t, testDB, agent.ID, "ACC-WD-REJECT")

	funding := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        agent.ID,
		"referenceType":  "commission_record",
		"referenceID":    "COM-WD-REJECT",
		"ledgerType":     "income",
		"amount":         80.0,
		"currency":       "CNY",
		"idempotencyKey": "ledger-income-wd-reject",
	})
	require.Equal(t, http.StatusCreated, funding.Code, funding.Body.String())

	createResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         agent.ID,
		"amount":          20.0,
		"currency":        "CNY",
		"channel":         "bank_transfer",
		"bankAccountName": "Reject Agent",
		"bankAccountNo":   "6222000000002",
		"bankName":        "Test Bank",
		"idempotencyKey":  "withdrawal-reject-flow-1",
	})
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	var request model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &request))

	reviewResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/review", map[string]any{
		"action": "reject",
		"remark": "risk blocked",
	})
	require.Equal(t, http.StatusOK, reviewResp.Code, reviewResp.Body.String())

	var reviewed model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(reviewResp.Body.Bytes(), &reviewed))
	require.Equal(t, model.WithdrawalStatusRejected, reviewed.Status)
	require.NotNil(t, reviewed.ReviewedAt)
	require.NotNil(t, reviewed.FailureLedgerID)

	var account model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).First(&account).Error)
	require.InDelta(t, 80.0, account.Balance, 0.001)
	require.InDelta(t, 0.0, account.FrozenBalance, 0.001)
	require.InDelta(t, 80.0, account.WithdrawableAmount, 0.001)

	var ledgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 3)
	require.Equal(t, model.LedgerTypeIncome, ledgers[0].LedgerType)
	require.Equal(t, model.LedgerTypeFreeze, ledgers[1].LedgerType)
	require.Equal(t, model.LedgerTypeUnfreeze, ledgers[2].LedgerType)
	require.Equal(t, request.RequestNo, ledgers[2].ReferenceID)
	require.Equal(t, "withdrawal-reject:"+request.IdempotencyKey, ledgers[2].IdempotencyKey)
}

func TestWithdrawalApproveThenPayoutSuccessAndFailure(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-WD-PAYOUT", "Withdrawal Payout Agent")
	createAgentAccountFixture(t, testDB, agent.ID, "ACC-WD-PAYOUT")

	funding := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        agent.ID,
		"referenceType":  "commission_record",
		"referenceID":    "COM-WD-PAYOUT",
		"ledgerType":     "income",
		"amount":         150.0,
		"currency":       "CNY",
		"idempotencyKey": "ledger-income-wd-payout",
	})
	require.Equal(t, http.StatusCreated, funding.Code, funding.Body.String())

	createResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         agent.ID,
		"amount":          30.0,
		"currency":        "CNY",
		"channel":         "bank_transfer",
		"bankAccountName": "Payout Agent",
		"bankAccountNo":   "6222000000008",
		"bankName":        "Test Bank",
		"idempotencyKey":  "withdrawal-payout-success-1",
	})
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	var request model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &request))
	require.Equal(t, 30.0, request.PayableAmount)
	assert.NotEmpty(t, request.BankAccountSnapshot)

	approveResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/review", map[string]any{
		"action": "approve",
		"remark": "approved for payout",
	})
	require.Equal(t, http.StatusOK, approveResp.Code, approveResp.Body.String())
	var approved model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(approveResp.Body.Bytes(), &approved))
	require.Equal(t, model.WithdrawalStatusApproved, approved.Status)
	require.Nil(t, approved.CompletedLedgerID)

	payoutResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/payout", map[string]any{
		"action":    "success",
		"remark":    "bank paid",
		"reference": "OUT-12345",
		"receiptPayload": map[string]any{
			"voucher": "receipt-1",
		},
	})
	require.Equal(t, http.StatusOK, payoutResp.Code, payoutResp.Body.String())
	var paid model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(payoutResp.Body.Bytes(), &paid))
	require.Equal(t, model.WithdrawalStatusPaid, paid.Status)
	require.NotNil(t, paid.CompletedLedgerID)
	require.NotNil(t, paid.PaidAt)
	require.Equal(t, "OUT-12345", paid.PayoutReference)
	assert.NotEmpty(t, paid.PayoutReceiptPayload)

	var account model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).First(&account).Error)
	require.InDelta(t, 120.0, account.Balance, 0.001)
	require.InDelta(t, 0.0, account.FrozenBalance, 0.001)
	require.InDelta(t, 120.0, account.WithdrawableAmount, 0.001)

	createFailResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         agent.ID,
		"amount":          20.0,
		"currency":        "CNY",
		"channel":         "bank_transfer",
		"bankAccountName": "Payout Agent",
		"bankAccountNo":   "6222000000009",
		"bankName":        "Test Bank",
		"idempotencyKey":  "withdrawal-payout-fail-1",
	})
	require.Equal(t, http.StatusCreated, createFailResp.Code, createFailResp.Body.String())
	var failRequest model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(createFailResp.Body.Bytes(), &failRequest))

	approveFailResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(failRequest.ID)+"/review", map[string]any{
		"action": "approve",
		"remark": "approved for payout fail path",
	})
	require.Equal(t, http.StatusOK, approveFailResp.Code, approveFailResp.Body.String())

	failResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(failRequest.ID)+"/payout", map[string]any{
		"action":    "fail",
		"remark":    "bank rejected",
		"reference": "OUT-FAIL-1",
	})
	require.Equal(t, http.StatusOK, failResp.Code, failResp.Body.String())
	var failed model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(failResp.Body.Bytes(), &failed))
	require.Equal(t, model.WithdrawalStatusFailed, failed.Status)
	require.NotNil(t, failed.FailureLedgerID)
	require.Equal(t, "OUT-FAIL-1", failed.PayoutReference)

	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).First(&account).Error)
	require.InDelta(t, 120.0, account.Balance, 0.001)
	require.InDelta(t, 0.0, account.FrozenBalance, 0.001)
	require.InDelta(t, 120.0, account.WithdrawableAmount, 0.001)

	var ledgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 6)
	require.Equal(t, model.LedgerTypeUnfreeze, ledgers[2].LedgerType)
	require.Equal(t, "withdrawal-complete-unfreeze:"+request.IdempotencyKey, ledgers[2].IdempotencyKey)
	require.Equal(t, model.LedgerTypeReverse, ledgers[3].LedgerType)
	require.Equal(t, "withdrawal-complete:"+request.IdempotencyKey, ledgers[3].IdempotencyKey)
	require.Equal(t, ledgers[3].ID, *paid.CompletedLedgerID)
	require.Equal(t, model.LedgerTypeFreeze, ledgers[4].LedgerType)
	require.Equal(t, model.LedgerTypeUnfreeze, ledgers[5].LedgerType)
	require.Equal(t, "withdrawal-fail:"+failRequest.IdempotencyKey, ledgers[5].IdempotencyKey)
	require.Equal(t, ledgers[5].ID, *failed.FailureLedgerID)
}

func TestWithdrawalReviewApproveIsIdempotent(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-WD-IDEMP", "Withdrawal Idempotent Agent")
	createAgentAccountFixture(t, testDB, agent.ID, "ACC-WD-IDEMP")

	funding := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        agent.ID,
		"referenceType":  "commission_record",
		"referenceID":    "COM-WD-IDEMP",
		"ledgerType":     "income",
		"amount":         90.0,
		"currency":       "CNY",
		"idempotencyKey": "ledger-income-wd-idemp",
	})
	require.Equal(t, http.StatusCreated, funding.Code, funding.Body.String())

	createResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         agent.ID,
		"amount":          25.0,
		"currency":        "CNY",
		"channel":         "bank_transfer",
		"bankAccountName": "Approve Agent",
		"bankAccountNo":   "6222000000003",
		"bankName":        "Test Bank",
		"idempotencyKey":  "withdrawal-approve-idempotent-1",
	})
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	var request model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &request))

	firstReview := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/review", map[string]any{
		"action": "approve",
		"remark": "first approval",
	})
	require.Equal(t, http.StatusOK, firstReview.Code, firstReview.Body.String())

	secondReview := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/review", map[string]any{
		"action": "approve",
		"remark": "duplicate approval retry",
	})
	require.Equal(t, http.StatusOK, secondReview.Code, secondReview.Body.String())

	var approvedAgain model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(secondReview.Body.Bytes(), &approvedAgain))
	require.Equal(t, model.WithdrawalStatusApproved, approvedAgain.Status)
	require.Nil(t, approvedAgain.CompletedLedgerID)

	var ledgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 2)
	require.Equal(t, model.LedgerTypeIncome, ledgers[0].LedgerType)
	require.Equal(t, model.LedgerTypeFreeze, ledgers[1].LedgerType)

	var account model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", agent.ID).First(&account).Error)
	require.InDelta(t, 90.0, account.Balance, 0.001)
	require.InDelta(t, 25.0, account.FrozenBalance, 0.001)
	require.InDelta(t, 65.0, account.WithdrawableAmount, 0.001)
}

func TestWithdrawalPhase3FeeTaxLifecycleAndTenantScope(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "P3-T1", Name: "Withdrawal Phase3 Tenant 1", Status: model.TenantStatusActive}
	tenantTwo := model.Tenant{Code: "WD3-T2", Name: "Withdrawal Phase3 Tenant 2", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	require.NoError(t, testDB.Create(&tenantTwo).Error)
	scopeTestUser(t, testDB, "admin", &tenantOne.ID, nil)
	scopeTestUser(t, testDB, "finance", &tenantOne.ID, nil)

	agentOne := createAgentFixture(t, testDB, "AG-WD3-1", "Withdrawal Phase3 Agent 1")
	agentTwo := createAgentFixture(t, testDB, "AG-WD3-2", "Withdrawal Phase3 Agent 2")
	require.NoError(t, testDB.Model(&agentOne).Update("tenant_id", tenantOne.ID).Error)
	require.NoError(t, testDB.Model(&agentTwo).Update("tenant_id", tenantTwo.ID).Error)
	require.NoError(t, testDB.First(&agentOne, agentOne.ID).Error)
	require.NoError(t, testDB.First(&agentTwo, agentTwo.ID).Error)
	createAgentAccountFixture(t, testDB, agentOne.ID, "ACC-WD3-1")
	createAgentAccountFixture(t, testDB, agentTwo.ID, "ACC-WD3-2")

	inScopeFunding := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{"agentID": agentOne.ID, "referenceType": "commission_record", "referenceID": "COM-WD3-1", "ledgerType": "income", "amount": 200.0, "currency": "CNY", "idempotencyKey": "ledger-income-wd3-1"})
	require.Equal(t, http.StatusCreated, inScopeFunding.Code, inScopeFunding.Body.String())
	outOfScopeFunding := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{"agentID": agentTwo.ID, "referenceType": "commission_record", "referenceID": "COM-WD3-2", "ledgerType": "income", "amount": 200.0, "currency": "CNY", "idempotencyKey": "ledger-income-wd3-2"})
	require.Equal(t, http.StatusNotFound, outOfScopeFunding.Code, outOfScopeFunding.Body.String())
	require.NoError(t, testDB.Model(&model.AgentAccount{}).Where("agent_id = ?", agentTwo.ID).Updates(map[string]any{"balance": 200.0, "available_balance": 200.0, "withdrawable_amount": 200.0}).Error)

	createInScope := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      tenantOne.Code,
		"agentID":         agentOne.ID,
		"amount":          100.0,
		"currency":        "CNY",
		"channel":         "bank_transfer",
		"bankAccountName": "Phase3 Agent 1",
		"bankAccountNo":   "6222000000010",
		"bankName":        "Test Bank",
		"idempotencyKey":  "withdrawal-phase3-scope-1",
	})
	require.Equal(t, http.StatusCreated, createInScope.Code, createInScope.Body.String())
	var inScope model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(createInScope.Body.Bytes(), &inScope))

	createOutScope := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      tenantTwo.Code,
		"agentID":         agentTwo.ID,
		"amount":          80.0,
		"currency":        "CNY",
		"channel":         "bank_transfer",
		"bankAccountName": "Phase3 Agent 2",
		"bankAccountNo":   "6222000000011",
		"bankName":        "Test Bank",
		"idempotencyKey":  "withdrawal-phase3-scope-2",
	})
	require.Equal(t, http.StatusNotFound, createOutScope.Code, createOutScope.Body.String())
	assertJSONErrorBody(t, createOutScope, "record not found")
	outScope := model.WithdrawalRequest{RequestNo: "WD-WD3-OUT", TenantCode: tenantTwo.Code, TenantID: &tenantTwo.ID, AgentID: agentTwo.ID, AccountID: accountIDForAgent(t, testDB, agentTwo.ID), Currency: "CNY", Amount: 80, Status: model.WithdrawalStatusPending, IdempotencyKey: "withdrawal-phase3-scope-2"}
	require.NoError(t, testDB.Create(&outScope).Error)
	require.NoError(t, testDB.Model(&model.AgentAccount{}).Where("agent_id = ?", agentTwo.ID).Updates(map[string]any{"frozen_balance": 80.0, "withdrawable_amount": 120.0}).Error)

	inScope.FeeAmount = 3.25
	inScope.TaxAmount = 6.75
	inScope.PayableAmount = 90.0
	require.NoError(t, testDB.Save(&inScope).Error)

	approveResp := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/withdrawals/"+jsonUint(inScope.ID)+"/review", map[string]any{
		"action": "approve",
		"remark": "phase3 approve",
	})
	require.Equal(t, http.StatusOK, approveResp.Code, approveResp.Body.String())
	var approved model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(approveResp.Body.Bytes(), &approved))
	require.Equal(t, model.WithdrawalStatusApproved, approved.Status)
	require.InDelta(t, 3.25, approved.FeeAmount, 0.001)
	require.InDelta(t, 6.75, approved.TaxAmount, 0.001)
	require.InDelta(t, 90.0, approved.PayableAmount, 0.001)
	require.Equal(t, "admin", approved.ReviewedBy)
	require.NotNil(t, approved.ReviewedAt)

	crossTenantReview := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/withdrawals/"+jsonUint(outScope.ID)+"/review", map[string]any{
		"action": "approve",
	})
	require.Equal(t, http.StatusNotFound, crossTenantReview.Code, crossTenantReview.Body.String())
	assertJSONErrorBody(t, crossTenantReview, "record not found")

	crossTenantPayout := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/withdrawals/"+jsonUint(outScope.ID)+"/payout", map[string]any{
		"action":    "success",
		"reference": "OUT-CROSS-1",
	})
	require.Equal(t, http.StatusNotFound, crossTenantPayout.Code, crossTenantPayout.Body.String())
	assertJSONErrorBody(t, crossTenantPayout, "record not found")

	returnResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(inScope.ID)+"/payout", map[string]any{
		"action":    "return",
		"remark":    "returned by bank",
		"reference": "OUT-RETURN-1",
		"receiptPayload": map[string]any{
			"voucher": "return-receipt-1",
			"reason":  "account mismatch",
		},
	})
	require.Equal(t, http.StatusOK, returnResp.Code, returnResp.Body.String())
	var returned model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(returnResp.Body.Bytes(), &returned))
	require.Equal(t, model.WithdrawalStatusReturned, returned.Status)
	require.NotNil(t, returned.FailureLedgerID)
	require.Equal(t, "OUT-RETURN-1", returned.PayoutReference)
	assert.NotEmpty(t, returned.PayoutReceiptPayload)
	assert.Contains(t, string(returned.PayoutReceiptPayload), "return-receipt-1")
	require.InDelta(t, 3.25, returned.FeeAmount, 0.001)
	require.InDelta(t, 6.75, returned.TaxAmount, 0.001)
	require.InDelta(t, 90.0, returned.PayableAmount, 0.001)

	var accountOne model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", agentOne.ID).First(&accountOne).Error)
	require.InDelta(t, 200.0, accountOne.Balance, 0.001)
	require.InDelta(t, 0.0, accountOne.FrozenBalance, 0.001)
	require.InDelta(t, 200.0, accountOne.WithdrawableAmount, 0.001)

	var ledgers []model.AgentAccountLedger
	require.NoError(t, testDB.Where("agent_id = ?", agentOne.ID).Order("id asc").Find(&ledgers).Error)
	require.Len(t, ledgers, 3)
	require.Equal(t, model.LedgerTypeIncome, ledgers[0].LedgerType)
	require.Equal(t, model.LedgerTypeFreeze, ledgers[1].LedgerType)
	require.Equal(t, model.LedgerTypeUnfreeze, ledgers[2].LedgerType)
	require.InDelta(t, 100.0, ledgers[2].Amount, 0.001)
	require.Equal(t, "withdrawal-return:"+inScope.IdempotencyKey, ledgers[2].IdempotencyKey)

	listResp := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/withdrawals?tenantCode="+tenantTwo.Code, nil)
	require.Equal(t, http.StatusOK, listResp.Code, listResp.Body.String())
	var listBody struct {
		Items []withdrawalListItem `json:"items"`
		Total int                  `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listBody))
	require.Zero(t, listBody.Total)
	for _, item := range listBody.Items {
		require.Equal(t, tenantOne.Code, item.TenantCode)
	}

	var accountTwo model.AgentAccount
	require.NoError(t, testDB.Where("agent_id = ?", agentTwo.ID).First(&accountTwo).Error)
	require.InDelta(t, 200.0, accountTwo.Balance, 0.001)
	require.InDelta(t, 80.0, accountTwo.FrozenBalance, 0.001)
	require.InDelta(t, 120.0, accountTwo.WithdrawableAmount, 0.001)

	var audits []model.OperationAuditLog
	require.NoError(t, testDB.Where("target_type = ?", "withdrawal_request").Order("id asc").Find(&audits).Error)
	require.NotEmpty(t, audits)
	var foundReturnAudit bool
	for _, audit := range audits {
		if audit.Action == "withdrawal_payout" && audit.TargetID == jsonUint(inScope.ID) && audit.Result == model.AuditResultSuccess {
			foundReturnAudit = true
			assert.Contains(t, string(audit.AfterPayload), "returned")
			assert.Contains(t, string(audit.AfterPayload), "return-receipt-1")
		}
	}
	require.True(t, foundReturnAudit)
}

func TestWithdrawalValidation(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	agent := createAgentFixture(t, testDB, "AG-WD-VAL", "Withdrawal Validate")
	createAgentAccountFixture(t, testDB, agent.ID, "ACC-WD-VAL")

	missingAmount := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"agentID":        agent.ID,
		"idempotencyKey": "wd-invalid-1",
	})
	require.Equal(t, http.StatusBadRequest, missingAmount.Code)
	assertJSONErrorBody(t, missingAmount, "amount must be greater than 0")

	badAgent := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         uint64(999999),
		"amount":          10,
		"currency":        "CNY",
		"idempotencyKey":  "wd-invalid-2",
		"bankAccountName": "Missing",
		"bankAccountNo":   "000",
		"bankName":        "Bank",
	})
	require.Equal(t, http.StatusNotFound, badAgent.Code)

	insufficient := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         agent.ID,
		"amount":          10,
		"currency":        "CNY",
		"idempotencyKey":  "wd-invalid-3",
		"bankAccountName": "Low Balance",
		"bankAccountNo":   "111",
		"bankName":        "Bank",
	})
	require.Equal(t, http.StatusBadRequest, insufficient.Code)
	assertJSONErrorBody(t, insufficient, "insufficient withdrawable amount")

	funding := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        agent.ID,
		"referenceType":  "commission_record",
		"referenceID":    "COM-WD-VAL",
		"ledgerType":     "income",
		"amount":         50,
		"currency":       "CNY",
		"idempotencyKey": "ledger-income-wd-val",
	})
	require.Equal(t, http.StatusCreated, funding.Code)

	createResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals", map[string]any{
		"tenantCode":      "t1",
		"agentID":         agent.ID,
		"amount":          20,
		"currency":        "CNY",
		"idempotencyKey":  "wd-valid-1",
		"bankAccountName": "Ok",
		"bankAccountNo":   "222",
		"bankName":        "Bank",
	})
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	var request model.WithdrawalRequest
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &request))

	invalidAction := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/review", map[string]any{
		"action": "hold",
	})
	require.Equal(t, http.StatusBadRequest, invalidAction.Code)
	assertJSONErrorBody(t, invalidAction, "action must be approve or reject")

	invalidList := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/withdrawals?agentID=bad-id", nil)
	require.Equal(t, http.StatusBadRequest, invalidList.Code)

	rejectResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/review", map[string]any{
		"action": "reject",
		"remark": "kyc failed",
	})
	require.Equal(t, http.StatusOK, rejectResp.Code, rejectResp.Body.String())

	repeatReview := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/withdrawals/"+jsonUint(request.ID)+"/review", map[string]any{
		"action": "approve",
	})
	require.Equal(t, http.StatusBadRequest, repeatReview.Code)
	assertJSONErrorBody(t, repeatReview, "withdrawal request is not pending")
}

func TestRiskAndReportValidation(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)
	_ = createAgentFixture(t, testDB, "AG-RISK-VALID", "Risk Validate")

	forbiddenRisk := performRequestWithToken(t, router, "operator", http.MethodGet, "/api/risk/agent-accounts", nil)
	require.Equal(t, http.StatusForbidden, forbiddenRisk.Code)

	badRisk := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/risk/agent-accounts?minFrozenAmount=bad", nil)
	require.Equal(t, http.StatusBadRequest, badRisk.Code)

	badRiskAgent := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/risk/agent-accounts?agentID=bad", nil)
	require.Equal(t, http.StatusBadRequest, badRiskAgent.Code)

	badAgentReport := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/agent-performance?agentID=bad", nil)
	require.Equal(t, http.StatusBadRequest, badAgentReport.Code)

	badTeamReportAgent := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/team-performance?agentID=bad", nil)
	require.Equal(t, http.StatusBadRequest, badTeamReportAgent.Code)

	badSettlementReport := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/game-settlement?currency=CNY12345678901234567", nil)
	require.Equal(t, http.StatusBadRequest, badSettlementReport.Code)

	badSettlementProgressReport := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/settlement-progress?currency=CNY12345678901234567", nil)
	require.Equal(t, http.StatusBadRequest, badSettlementProgressReport.Code)

	badTeamReportCurrency := performRequestWithToken(t, router, "finance", http.MethodGet, "/api/report/team-performance?currency=CNY12345678901234567", nil)
	require.Equal(t, http.StatusBadRequest, badTeamReportCurrency.Code)

	missingAccountLedger := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        uint64(999999),
		"referenceType":  "risk_case",
		"referenceID":    "RISK-404",
		"ledgerType":     "freeze",
		"amount":         10,
		"idempotencyKey": "missing-account-ledger",
	})
	require.Equal(t, http.StatusNotFound, missingAccountLedger.Code)
}

func TestRiskCaseWorkflowTracksStatusAndConfigCenterFiltersByTenant(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	seedRBAC(t, testDB)

	tenantOne := model.Tenant{Code: "t1", Name: "Tenant One", DisplayName: "Tenant One", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantOne).Error)
	tenantTwo := model.Tenant{Code: "t2", Name: "Tenant Two", DisplayName: "Tenant Two", Status: model.TenantStatusActive}
	require.NoError(t, testDB.Create(&tenantTwo).Error)

	tenantOneBrand := model.Brand{TenantID: tenantOne.ID, Code: "b1", Name: "Brand One", DisplayName: "Brand One", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&tenantOneBrand).Error)
	tenantTwoBrand := model.Brand{TenantID: tenantTwo.ID, Code: "b2", Name: "Brand Two", DisplayName: "Brand Two", Status: model.BrandStatusActive}
	require.NoError(t, testDB.Create(&tenantTwoBrand).Error)

	agent := createAgentFixture(t, testDB, "AG-RISK-WF", "Risk Workflow Agent")
	createAgentAccountFixture(t, testDB, agent.ID, "ACC-RISK-WF")

	funding := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/agent-accounts/ledger", map[string]any{
		"agentID":        agent.ID,
		"referenceType":  "commission_record",
		"referenceID":    "COM-RISK-WF-1",
		"ledgerType":     "income",
		"amount":         200,
		"currency":       "CNY",
		"idempotencyKey": "ledger-income-risk-wf",
	})
	require.Equal(t, http.StatusCreated, funding.Code, funding.Body.String())

	createResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
		"caseNo":   "RISK-WF-001",
		"agentID":  agent.ID,
		"reason":   "review me",
		"freeze":   true,
		"amount":   55.5,
		"currency": "CNY",
		"remark":   "created from test",
	})
	require.Equal(t, http.StatusCreated, createResp.Code, createResp.Body.String())
	var created struct {
		CaseNo       string                    `json:"caseNo"`
		Status       string                    `json:"status"`
		Frozen       bool                      `json:"frozen"`
		FrozenLedger *model.AgentAccountLedger `json:"frozenLedger"`
	}
	require.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &created))
	require.Equal(t, "RISK-WF-001", created.CaseNo)
	require.Equal(t, "pending", created.Status)
	require.True(t, created.Frozen)
	require.NotNil(t, created.FrozenLedger)

	releaseResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/RISK-WF-001/review", map[string]any{
		"action": "release",
		"remark": "release for clear",
	})
	require.Equal(t, http.StatusOK, releaseResp.Code, releaseResp.Body.String())
	var released struct {
		CaseNo string                    `json:"caseNo"`
		Action string                    `json:"action"`
		Status string                    `json:"status"`
		Ledger *model.AgentAccountLedger `json:"ledger"`
	}
	require.NoError(t, json.Unmarshal(releaseResp.Body.Bytes(), &released))
	require.Equal(t, "release", released.Action)
	require.Equal(t, "released", released.Status)
	require.NotNil(t, released.Ledger)

	confirmAfterRelease := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/RISK-WF-001/review", map[string]any{
		"action": "confirm",
	})
	require.Equal(t, http.StatusBadRequest, confirmAfterRelease.Code)
	assertJSONErrorBody(t, confirmAfterRelease, "risk case already reviewed")

	secondCreateResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases", map[string]any{
		"caseNo":   "RISK-WF-002",
		"agentID":  agent.ID,
		"reason":   "confirm path",
		"freeze":   true,
		"amount":   12.5,
		"currency": "CNY",
		"remark":   "second case",
	})
	require.Equal(t, http.StatusCreated, secondCreateResp.Code, secondCreateResp.Body.String())

	confirmResp := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/RISK-WF-002/review", map[string]any{
		"action": "confirm",
		"remark": "confirmed by finance",
	})
	require.Equal(t, http.StatusOK, confirmResp.Code, confirmResp.Body.String())
	var confirmed struct {
		CaseNo string                    `json:"caseNo"`
		Action string                    `json:"action"`
		Status string                    `json:"status"`
		Ledger *model.AgentAccountLedger `json:"ledger"`
	}
	require.NoError(t, json.Unmarshal(confirmResp.Body.Bytes(), &confirmed))
	require.Equal(t, "confirmed", confirmed.Status)
	require.Nil(t, confirmed.Ledger)

	releaseAfterConfirm := performJSONWithToken(t, router, "finance", http.MethodPost, "/api/risk/cases/RISK-WF-002/review", map[string]any{
		"action": "release",
	})
	require.Equal(t, http.StatusBadRequest, releaseAfterConfirm.Code)
	assertJSONErrorBody(t, releaseAfterConfirm, "risk case already reviewed")

	createPlatformConfig := performJSONWithToken(t, router, "admin", http.MethodPost, "/api/platform-configs", map[string]any{
		"tenantID":    tenantOne.ID,
		"brandID":     tenantOneBrand.ID,
		"key":         "risk.review.mode",
		"value":       map[string]any{"enabled": true},
		"description": "tenant one config",
	})
	require.Equal(t, http.StatusCreated, createPlatformConfig.Code, createPlatformConfig.Body.String())
	var configBody struct {
		ID       uint64  `json:"id"`
		TenantID *uint64 `json:"tenantID"`
		BrandID  *uint64 `json:"brandID"`
		Key      string  `json:"key"`
	}
	require.NoError(t, json.Unmarshal(createPlatformConfig.Body.Bytes(), &configBody))
	require.NotZero(t, configBody.ID)
	require.NotNil(t, configBody.TenantID)
	require.Equal(t, tenantOne.ID, *configBody.TenantID)
	operatorConfig := performJSONWithToken(t, router, "operator", http.MethodPost, "/api/platform-configs", map[string]any{"key": "blocked", "value": map[string]any{"x": 1}})
	require.Equal(t, http.StatusForbidden, operatorConfig.Code)

	listResp := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/platform-configs?tenantCode=t1", nil)
	require.Equal(t, http.StatusOK, listResp.Code, listResp.Body.String())
	var listed struct {
		Items []struct {
			ID         uint64 `json:"id"`
			TenantCode string `json:"tenantCode"`
			Key        string `json:"key"`
		} `json:"items"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &listed))
	require.Equal(t, 1, listed.Total)
	require.Equal(t, "t1", listed.Items[0].TenantCode)
	require.Equal(t, "risk.review.mode", listed.Items[0].Key)

	emptyScopedList := performRequestWithToken(t, router, "admin", http.MethodGet, "/api/platform-configs?tenantCode=t2", nil)
	require.Equal(t, http.StatusOK, emptyScopedList.Code)
	require.NoError(t, json.Unmarshal(emptyScopedList.Body.Bytes(), &listed))
	require.Zero(t, listed.Total)

	updateResp := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/platform-configs/"+jsonUint(configBody.ID), map[string]any{
		"tenantID":    tenantOne.ID,
		"brandID":     tenantOneBrand.ID,
		"key":         "risk.review.mode",
		"value":       map[string]any{"enabled": false, "mode": "manual"},
		"description": "tenant one config updated",
	})
	require.Equal(t, http.StatusOK, updateResp.Code, updateResp.Body.String())
	var updated model.PlatformConfig
	require.NoError(t, json.Unmarshal(updateResp.Body.Bytes(), &updated))
	require.Equal(t, configBody.ID, updated.ID)
	require.Equal(t, "admin", updated.UpdatedBy)
	assert.Contains(t, string(updated.Value), "\"manual\"")

	immutableResp := performJSONWithToken(t, router, "admin", http.MethodPut, "/api/platform-configs/"+jsonUint(configBody.ID), map[string]any{
		"tenantID":    tenantOne.ID,
		"brandID":     tenantTwoBrand.ID,
		"key":         "risk.review.mode.changed",
		"value":       map[string]any{"enabled": true},
		"description": "bad update",
	})
	require.Equal(t, http.StatusBadRequest, immutableResp.Code, immutableResp.Body.String())
	assertJSONErrorBody(t, immutableResp, "platform config key and scope cannot be changed")

	var audits []model.OperationAuditLog
	require.NoError(t, testDB.Where("action IN ?", []string{"risk_case_create", "risk_case_review", "platform_config_create", "platform_config_update"}).Order("id asc").Find(&audits).Error)
	require.GreaterOrEqual(t, len(audits), 5)
}

func TestPhase2RuleHitPriorityResolution(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	_ = router
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-RULE-PRIORITY", "Rule Priority Agent")
	game := createGameFixture(t, testDB, "GAME-RULE-PRIORITY", true)
	at := time.Date(2026, 4, 22, 13, 30, 0, 0, time.UTC)

	platformDefault := createRuleSnapshotFixture(t, testDB, ruleSnapshotFixture{
		ruleID:        1001,
		ruleVersion:   1,
		ruleUniqueKey: "platform-default",
		scope:         model.RuleScopePlatform,
		priority:      10,
		effectiveFrom: at.Add(-48 * time.Hour),
		publishedAt:   at.Add(-47 * time.Hour),
	})
	gameDefault := createRuleSnapshotFixture(t, testDB, ruleSnapshotFixture{
		ruleID:        1002,
		ruleVersion:   1,
		ruleUniqueKey: "game-default",
		scope:         model.RuleScopeGame,
		gameID:        &game.ID,
		priority:      10,
		effectiveFrom: at.Add(-36 * time.Hour),
		publishedAt:   at.Add(-35 * time.Hour),
	})
	agentDefault := createRuleSnapshotFixture(t, testDB, ruleSnapshotFixture{
		ruleID:        1003,
		ruleVersion:   1,
		ruleUniqueKey: "agent-default",
		scope:         model.RuleScopeAgent,
		agentID:       &agent.ID,
		priority:      10,
		effectiveFrom: at.Add(-24 * time.Hour),
		publishedAt:   at.Add(-23 * time.Hour),
	})
	agentGameSpecific := createRuleSnapshotFixture(t, testDB, ruleSnapshotFixture{
		ruleID:        1004,
		ruleVersion:   1,
		ruleUniqueKey: "agent-game-specific",
		scope:         model.RuleScopeAgentGame,
		agentID:       &agent.ID,
		gameID:        &game.ID,
		priority:      10,
		effectiveFrom: at.Add(-12 * time.Hour),
		publishedAt:   at.Add(-11 * time.Hour),
	})

	selected, err := FindApplicableRuleSnapshotForTest(testDB, agent.ID, game.ID, at)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, agentGameSpecific.ID, selected.ID)

	require.NoError(t, testDB.Delete(&model.RuleSnapshot{}, agentGameSpecific.ID).Error)
	selected, err = FindApplicableRuleSnapshotForTest(testDB, agent.ID, game.ID, at)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, agentDefault.ID, selected.ID)

	require.NoError(t, testDB.Delete(&model.RuleSnapshot{}, agentDefault.ID).Error)
	selected, err = FindApplicableRuleSnapshotForTest(testDB, agent.ID, game.ID, at)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, gameDefault.ID, selected.ID)

	require.NoError(t, testDB.Delete(&model.RuleSnapshot{}, gameDefault.ID).Error)
	selected, err = FindApplicableRuleSnapshotForTest(testDB, agent.ID, game.ID, at)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, platformDefault.ID, selected.ID)
}

func TestPhase2RuleHitTieBreakersUseEffectiveFromThenVersionDesc(t *testing.T) {
	router, testDB := newIsolatedTestRouter(t)
	_ = router
	seedRBAC(t, testDB)

	agent := createAgentFixture(t, testDB, "AG-RULE-TIE", "Rule Tie Agent")
	game := createGameFixture(t, testDB, "GAME-RULE-TIE", true)
	at := time.Date(2026, 4, 22, 14, 0, 0, 0, time.UTC)
	olderEffective := at.Add(-6 * time.Hour)
	newerEffective := at.Add(-3 * time.Hour)

	older := createRuleSnapshotFixture(t, testDB, ruleSnapshotFixture{
		ruleID:        2001,
		ruleVersion:   2,
		ruleUniqueKey: "tie-older-effective",
		scope:         model.RuleScopeAgentGame,
		agentID:       &agent.ID,
		gameID:        &game.ID,
		priority:      20,
		effectiveFrom: olderEffective,
		publishedAt:   olderEffective.Add(30 * time.Minute),
	})
	newerLowerVersion := createRuleSnapshotFixture(t, testDB, ruleSnapshotFixture{
		ruleID:        2002,
		ruleVersion:   1,
		ruleUniqueKey: "tie-newer-effective-lower-version",
		scope:         model.RuleScopeAgentGame,
		agentID:       &agent.ID,
		gameID:        &game.ID,
		priority:      20,
		effectiveFrom: newerEffective,
		publishedAt:   newerEffective.Add(15 * time.Minute),
	})
	newerHigherVersion := createRuleSnapshotFixture(t, testDB, ruleSnapshotFixture{
		ruleID:        2003,
		ruleVersion:   3,
		ruleUniqueKey: "tie-newer-effective-higher-version",
		scope:         model.RuleScopeAgentGame,
		agentID:       &agent.ID,
		gameID:        &game.ID,
		priority:      20,
		effectiveFrom: newerEffective,
		publishedAt:   newerEffective.Add(45 * time.Minute),
	})

	selected, err := FindApplicableRuleSnapshotForTest(testDB, agent.ID, game.ID, at)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, newerHigherVersion.ID, selected.ID)

	require.NoError(t, testDB.Delete(&model.RuleSnapshot{}, newerHigherVersion.ID).Error)
	selected, err = FindApplicableRuleSnapshotForTest(testDB, agent.ID, game.ID, at)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, newerLowerVersion.ID, selected.ID)

	require.NoError(t, testDB.Delete(&model.RuleSnapshot{}, newerLowerVersion.ID).Error)
	selected, err = FindApplicableRuleSnapshotForTest(testDB, agent.ID, game.ID, at)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, older.ID, selected.ID)
}

func marshalJSON(t *testing.T, v any) string {
	return string(marshalJSONBytes(v))
}

func marshalJSONBytes(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

var _ = datatypes.JSON{}

func collectAgentIDs(items []model.Agent) []uint64 {
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func collectPlayerIDs(items []model.Player) []uint64 {
	ids := make([]uint64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func collectOrderNos(items []model.RechargeOrder) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.OrderNo)
	}
	return values
}

func collectCommissionNos(items []model.CommissionRecord) []string {
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, item.RecordNo)
	}
	return values
}
