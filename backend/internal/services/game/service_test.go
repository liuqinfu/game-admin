package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"game-admin/backend/internal/domain/model"
	sharedsvc "game-admin/backend/internal/services/shared"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGameLifecycleAndAccessPublishEvents(t *testing.T) {
	t.Parallel()

	db := openGameTestDB(t)
	service := NewService(db)
	scope := sharedsvc.Scope{}

	game, credential, err := service.Create(scope, CreateInput{
		GameCode: "GM-1",
		Name:     "Game 1",
		Status:   model.GameStatusDraft,
	}, "tester")
	require.NoError(t, err)
	require.NotNil(t, credential)

	_, updatedGame, err := service.Update(scope, game.ID, UpdateInput{Name: "Game 1+", Status: model.GameStatusOnline})
	require.NoError(t, err)
	require.Equal(t, model.GameStatusOnline, updatedGame.Status)

	_, statusGame, err := service.UpdateStatus(scope, game.ID, model.GameStatusOffline)
	require.NoError(t, err)
	require.Equal(t, model.GameStatusOffline, statusGame.Status)

	rotated, err := service.RotateIntegrationKey(scope, game.ID, credential.ID)
	require.NoError(t, err)
	require.NotEmpty(t, rotated.SecretKey)

	agent := model.Agent{AgentNo: "AG-GAME-1", Name: "Agent 1", Status: model.AgentStatusActive, Currency: "CNY"}
	require.NoError(t, db.Create(&agent).Error)
	access, err := service.UpsertAccess(scope, AccessInput{AgentID: agent.ID, GameID: game.ID, Status: model.AccessStatusEnabled}, "tester")
	require.NoError(t, err)
	require.NotNil(t, access)
	_, err = service.RevokeAccess(scope, agent.ID, game.ID)
	require.NoError(t, err)

	var events []model.DomainEvent
	require.NoError(t, db.Order("id asc").Find(&events).Error)
	require.Len(t, events, 6)
	require.Equal(t, "game.created", events[0].EventType)
	require.Equal(t, "game.updated", events[1].EventType)
	require.Equal(t, "game.status_changed", events[2].EventType)
	require.Equal(t, "game.credential.rotated", events[3].EventType)
	require.Equal(t, "game.access.upserted", events[4].EventType)
	require.Equal(t, "game.access.revoked", events[5].EventType)
}

func TestAuthenticateOpenAPI(t *testing.T) {
	t.Parallel()

	db := openGameTestDB(t)
	service := NewService(db)
	scope := sharedsvc.Scope{}

	game, _, err := service.Create(scope, CreateInput{
		GameCode: "GM-AUTH-1",
		Name:     "Auth Game",
		Status:   model.GameStatusOnline,
	}, "tester")
	require.NoError(t, err)

	credential := model.GameIntegrationKey{
		GameID:           game.ID,
		AccessKey:        "ga_live_auth_test",
		SecretCiphertext: "gsk_live_auth_test",
		Status:           model.GameIntegrationKeyStatusActive,
		Scopes:           jsonStringArray(defaultGameOpenAPIScopes()),
	}
	require.NoError(t, db.Create(&credential).Error)

	timestamp := time.Now().UTC().Format(time.RFC3339)
	body := []byte(`{"ping":"pong"}`)
	signature := signGameOpenAPITestRequest(httpMethodPost, "/openapi/game/players", "", timestamp, "nonce-1", body, credential.SecretCiphertext)

	result, err := service.AuthenticateOpenAPI(OpenAPIAuthInput{
		AccessKey:      credential.AccessKey,
		TimestampValue: timestamp,
		Nonce:          "nonce-1",
		Signature:      signature,
		Method:         httpMethodPost,
		Path:           "/openapi/game/players",
		RawQuery:       "",
		Body:           body,
	})
	require.NoError(t, err)
	require.Equal(t, credential.ID, result.Credential.ID)
	require.Equal(t, game.ID, result.Game.ID)
	require.Contains(t, result.Scopes, "player:create")
}

const httpMethodPost = "POST"

func openGameTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "game.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(model.Phase1Models...))
	return db
}

func signGameOpenAPITestRequest(method, path, rawQuery, timestampValue, nonce string, body []byte, secret string) string {
	bodyHash := sha256.Sum256(body)
	payload := method + "\n" + path + "\n" + rawQuery + "\n" + timestampValue + "\n" + nonce + "\n" + hex.EncodeToString(bodyHash[:])
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
