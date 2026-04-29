package game

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/eventbus"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

type ListFilter struct {
	Status  string
	Keyword string
}

type CreateInput struct {
	TenantID    *uint64
	BrandID     *uint64
	GameCode    string
	Name        string
	Vendor      string
	Category    string
	Status      model.GameStatus
	IsAgentable *bool
	Sort        int32
	Remark      string
}

type UpdateInput struct {
	GameCode    string
	Name        string
	Vendor      string
	Category    string
	Status      model.GameStatus
	IsAgentable *bool
	Sort        int32
	Remark      string
}

type CredentialView struct {
	ID         uint64
	GameID     uint64
	Name       string
	AccessKey  string
	SecretKey  string
	Status     model.GameIntegrationKeyStatus
	Scopes     []string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RotatedAt  *time.Time
	ExpiresAt  *time.Time
	Remark     string
}

type AccessInput struct {
	AgentID uint64
	GameID  uint64
	Status  model.AccessStatus
	Remark  string
}

type AccessListItem struct {
	model.AgentGameAccess
	AgentName string
	GameCode  string
	GameName  string
}

type AccessFilter struct {
	AgentID string
	GameID  string
	Status  string
}

type OpenAPIAccessFilter struct {
	AgentID string
	Status  string
}

type OpenAPIAuthInput struct {
	AccessKey      string
	TimestampValue string
	Nonce          string
	Signature      string
	Method         string
	Path           string
	RawQuery       string
	Body           []byte
}

type OpenAPIAuthResult struct {
	Credential model.GameIntegrationKey
	Game       model.Game
	Scopes     []string
}

func NewService(db *gorm.DB) *Service { return &Service{db: db, repo: newRepository(db)} }

func (s *Service) List(scope shared.Scope, filter ListFilter) ([]model.Game, error) {
	return s.repo.ListGames(scope, filter)
}

func (s *Service) Create(scope shared.Scope, input CreateInput, createdBy string) (model.Game, *CredentialView, error) {
	tenantID, brandID, err := shared.ResolveTenantBrandScope(s.db, scope, input.TenantID, input.BrandID, false)
	if err != nil {
		return model.Game{}, nil, err
	}
	isAgentable := true
	if input.IsAgentable != nil {
		isAgentable = *input.IsAgentable
	}
	game := model.Game{
		TenantID:    tenantID,
		BrandID:     brandID,
		GameCode:    input.GameCode,
		Name:        input.Name,
		Vendor:      input.Vendor,
		Category:    input.Category,
		Status:      defaultGameStatus(input.Status),
		IsAgentable: isAgentable,
		Sort:        input.Sort,
		Remark:      input.Remark,
	}
	applyGameStatus(&game, game.Status, time.Now().UTC())
	if err := s.repo.CreateGame(&game); err != nil {
		return model.Game{}, nil, err
	}
	credential, secret, err := s.createDefaultGameIntegrationKey(game, createdBy)
	if err != nil {
		return model.Game{}, nil, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventGameCreated,
		AggregateType:  "game",
		AggregateID:    strconv.FormatUint(game.ID, 10),
		TenantID:       game.TenantID,
		BrandID:        game.BrandID,
		OccurredAt:     game.CreatedAt,
		Producer:       "game-service",
		IdempotencyKey: "game.created:" + game.GameCode,
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        gameCreatedPayload(game, credential.ID),
	}); err != nil {
		return model.Game{}, nil, err
	}
	return game, s.gameCredentialView(*credential, secret), nil
}

func (s *Service) Update(scope shared.Scope, id uint64, input UpdateInput) (model.Game, model.Game, error) {
	game, err := s.repo.FindGame(id)
	if err != nil {
		return model.Game{}, model.Game{}, err
	}
	if !shared.MatchesScopedRecord(scope, game.TenantID, game.BrandID) {
		return model.Game{}, model.Game{}, gorm.ErrRecordNotFound
	}
	before := game
	if input.GameCode != "" {
		game.GameCode = input.GameCode
	}
	if input.Name != "" {
		game.Name = input.Name
	}
	if input.Vendor != "" {
		game.Vendor = input.Vendor
	}
	if input.Category != "" {
		game.Category = input.Category
	}
	if input.Status != "" {
		if err := validateGameStatusTransition(game.Status, input.Status); err != nil {
			return model.Game{}, model.Game{}, err
		}
		applyGameStatus(&game, input.Status, time.Now().UTC())
	}
	if input.IsAgentable != nil {
		game.IsAgentable = *input.IsAgentable
	}
	game.Sort = input.Sort
	if input.Remark != "" {
		game.Remark = input.Remark
	}
	if err := s.repo.SaveGame(&game); err != nil {
		return model.Game{}, model.Game{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventGameUpdated,
		AggregateType:  "game",
		AggregateID:    strconv.FormatUint(game.ID, 10),
		TenantID:       game.TenantID,
		BrandID:        game.BrandID,
		OccurredAt:     game.UpdatedAt,
		Producer:       "game-service",
		IdempotencyKey: "game.updated:" + strconv.FormatUint(game.ID, 10) + ":" + strconv.FormatInt(game.UpdatedAt.UnixNano(), 10),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        gameUpdatedPayload(before, game),
	}); err != nil {
		return model.Game{}, model.Game{}, err
	}
	return before, game, nil
}

func (s *Service) UpdateStatus(scope shared.Scope, id uint64, status model.GameStatus) (model.Game, model.Game, error) {
	game, err := s.repo.FindGame(id)
	if err != nil {
		return model.Game{}, model.Game{}, err
	}
	if !shared.MatchesScopedRecord(scope, game.TenantID, game.BrandID) {
		return model.Game{}, model.Game{}, gorm.ErrRecordNotFound
	}
	before := game
	if err := validateGameStatusTransition(game.Status, status); err != nil {
		return model.Game{}, model.Game{}, err
	}
	applyGameStatus(&game, status, time.Now().UTC())
	if err := s.repo.SaveGame(&game); err != nil {
		return model.Game{}, model.Game{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventGameStatusChanged,
		AggregateType:  "game",
		AggregateID:    strconv.FormatUint(game.ID, 10),
		TenantID:       game.TenantID,
		BrandID:        game.BrandID,
		OccurredAt:     game.UpdatedAt,
		Producer:       "game-service",
		IdempotencyKey: "game.status:" + strconv.FormatUint(game.ID, 10) + ":" + string(game.Status),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        gameStatusChangedPayload(before, game),
	}); err != nil {
		return model.Game{}, model.Game{}, err
	}
	return before, game, nil
}

func (s *Service) Delete(scope shared.Scope, id uint64) (model.Game, error) {
	game, err := s.repo.FindGame(id)
	if err != nil {
		return model.Game{}, err
	}
	if !shared.MatchesScopedRecord(scope, game.TenantID, game.BrandID) {
		return model.Game{}, gorm.ErrRecordNotFound
	}
	if err := s.ensureDeleteReferencesClear([]deleteReferenceCheck{
		{model: &model.GameIntegrationKey{}, query: "game_id = ?", args: []any{game.ID}, message: "game has active integration keys"},
		{model: &model.AgentGameAccess{}, query: "game_id = ?", args: []any{game.ID}, message: "game has active agent access grants"},
		{model: &model.Player{}, query: "register_game_id = ?", args: []any{game.ID}, message: "game has active registered players"},
		{model: &model.RechargeOrder{}, query: "game_id = ?", args: []any{game.ID}, message: "game has active recharge orders"},
		{model: &model.CommissionRecord{}, query: "game_id = ?", args: []any{game.ID}, message: "game has active commission records"},
		{model: &model.CommissionRule{}, query: "game_id = ?", args: []any{game.ID}, message: "game has active commission rules"},
		{model: &model.RuleSnapshot{}, query: "game_id = ?", args: []any{game.ID}, message: "game has active rule snapshots"},
	}); err != nil {
		return model.Game{}, err
	}
	if err := s.repo.WithTx(func(tx *gorm.DB) error {
		if err := s.repo.CreateArchivedGameAndDeleteTx(tx, &game); err != nil {
			return err
		}
		_, err := eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventGameDeleted,
			AggregateType:  "game",
			AggregateID:    strconv.FormatUint(game.ID, 10),
			TenantID:       game.TenantID,
			BrandID:        game.BrandID,
			OccurredAt:     time.Now().UTC(),
			Producer:       "game-service",
			IdempotencyKey: "game.deleted:" + strconv.FormatUint(game.ID, 10),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        gameDeletedPayload(game),
		})
		return err
	}); err != nil {
		return model.Game{}, err
	}
	return game, nil
}

func (s *Service) ListIntegrationKeys(scope shared.Scope, gameID uint64) ([]*CredentialView, model.Game, error) {
	game, err := s.loadScopedGame(scope, gameID)
	if err != nil {
		return nil, model.Game{}, err
	}
	credentials, err := s.repo.ListIntegrationKeys(game)
	if err != nil {
		return nil, model.Game{}, err
	}
	items := make([]*CredentialView, 0, len(credentials))
	for _, credential := range credentials {
		items = append(items, s.gameCredentialView(credential, ""))
	}
	return items, game, nil
}

func (s *Service) RotateIntegrationKey(scope shared.Scope, gameID, keyID uint64) (*CredentialView, error) {
	game, err := s.loadScopedGame(scope, gameID)
	if err != nil {
		return nil, err
	}
	credential, err := s.repo.FindIntegrationKey(game, keyID)
	if err != nil {
		return nil, err
	}
	secret, err := generateToken("gsk_live", 32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if err := s.repo.UpdateIntegrationKeySecret(credential.ID, secret, now); err != nil {
		return nil, err
	}
	credential.SecretCiphertext = secret
	credential.RotatedAt = &now
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventGameCredentialRotated,
		AggregateType:  "game_integration_key",
		AggregateID:    strconv.FormatUint(credential.ID, 10),
		TenantID:       credential.TenantID,
		BrandID:        credential.BrandID,
		OccurredAt:     now,
		Producer:       "game-service",
		IdempotencyKey: "game.credential.rotated:" + strconv.FormatUint(credential.ID, 10) + ":" + strconv.FormatInt(now.UnixNano(), 10),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        gameCredentialRotatedPayload(credential),
	}); err != nil {
		return nil, err
	}
	return s.gameCredentialView(credential, secret), nil
}

func (s *Service) ListAccess(scope shared.Scope, filter AccessFilter) ([]AccessListItem, error) {
	return s.repo.ListAccess(scope, filter)
}

func (s *Service) ListAccessForGame(game model.Game, filter OpenAPIAccessFilter) ([]AccessListItem, error) {
	return s.repo.ListAccessForGame(game, filter)
}

func (s *Service) AuthenticateOpenAPI(input OpenAPIAuthInput) (OpenAPIAuthResult, error) {
	accessKey := strings.TrimSpace(input.AccessKey)
	timestampValue := strings.TrimSpace(input.TimestampValue)
	nonce := strings.TrimSpace(input.Nonce)
	signature := strings.TrimSpace(input.Signature)
	if accessKey == "" || timestampValue == "" || nonce == "" || signature == "" {
		return OpenAPIAuthResult{}, errors.New("game openapi signature headers are required")
	}
	timestamp, err := time.Parse(time.RFC3339, timestampValue)
	if err != nil {
		return OpenAPIAuthResult{}, errors.New("invalid game openapi timestamp")
	}
	if drift := time.Since(timestamp); drift > 5*time.Minute || drift < -5*time.Minute {
		return OpenAPIAuthResult{}, errors.New("game openapi timestamp expired")
	}
	credential, err := s.repo.FindIntegrationKeyByAccessKey(accessKey)
	if err != nil {
		return OpenAPIAuthResult{}, errors.New("invalid game access key")
	}
	if credential.Status != model.GameIntegrationKeyStatusActive {
		return OpenAPIAuthResult{}, errors.New("game access key is not active")
	}
	if credential.ExpiresAt != nil && !credential.ExpiresAt.After(time.Now().UTC()) {
		return OpenAPIAuthResult{}, errors.New("game access key is expired")
	}
	game, err := s.repo.FindGame(credential.GameID)
	if err != nil {
		return OpenAPIAuthResult{}, errors.New("game access key is invalid")
	}
	if !validGameOpenAPISignature(input.Method, input.Path, input.RawQuery, timestampValue, nonce, input.Body, credential.SecretCiphertext, signature) {
		return OpenAPIAuthResult{}, errors.New("invalid game openapi signature")
	}
	_ = s.repo.TouchIntegrationKeyLastUsed(credential.ID, time.Now().UTC())
	return OpenAPIAuthResult{
		Credential: credential,
		Game:       game,
		Scopes:     stringSliceFromJSON(credential.Scopes),
	}, nil
}

func (s *Service) UpsertAccess(scope shared.Scope, input AccessInput, grantedBy string) (*model.AgentGameAccess, error) {
	if err := s.ensureAgentGameAccessPayloadInScope(scope, input); err != nil {
		return nil, err
	}
	status := input.Status
	if status == "" {
		status = model.AccessStatusEnabled
	}
	if status != model.AccessStatusEnabled && status != model.AccessStatusDisabled {
		return nil, errors.New("status is invalid")
	}
	var access model.AgentGameAccess
	err := s.repo.WithTx(func(tx *gorm.DB) error {
		agent, err := s.loadAgent(tx, input.AgentID)
		if err != nil {
			return err
		}
		game, err := s.loadGame(tx, input.GameID)
		if err != nil {
			return err
		}
		if !game.IsAgentable {
			return errors.New("game is not agent-accessible")
		}
		if err := shared.EnsureSameScope("game", agent.TenantID, agent.BrandID, game.TenantID, game.BrandID); err != nil {
			return err
		}
		now := time.Now().UTC()
		if current, err := s.repo.FindAgentGameAccessTx(tx, input.AgentID, input.GameID); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			access = model.AgentGameAccess{
				TenantID:      agent.TenantID,
				BrandID:       agent.BrandID,
				AgentID:       input.AgentID,
				GameID:        input.GameID,
				Status:        status,
				GrantedBy:     grantedBy,
				GrantedAt:     now,
				EffectiveFrom: now,
			}
			if input.Remark != "" {
				access.SettlementMemo = input.Remark
			}
			if err := s.repo.CreateAgentGameAccessTx(tx, &access); err != nil {
				return err
			}
			_, err = eventbus.Publish(tx, eventbus.PublishInput{
				EventType:      eventbus.EventGameAccessUpserted,
				AggregateType:  "agent_game_access",
				AggregateID:    strconv.FormatUint(access.ID, 10),
				TenantID:       access.TenantID,
				BrandID:        access.BrandID,
				OccurredAt:     now,
				Producer:       "game-service",
				IdempotencyKey: "game.access.upserted:" + strconv.FormatUint(access.AgentID, 10) + ":" + strconv.FormatUint(access.GameID, 10) + ":" + string(access.Status),
				Consumers:      eventbus.ConsumersDataPlatformSync(),
				Payload:        gameAccessPayload(access),
			})
			return err
		} else {
			access = current
		}
		access.Status = status
		access.GrantedBy = grantedBy
		access.GrantedAt = now
		access.EffectiveFrom = now
		access.EffectiveTo = nil
		access.SettlementMemo = input.Remark
		access.TenantID = agent.TenantID
		access.BrandID = agent.BrandID
		if err := s.repo.SaveAgentGameAccessTx(tx, &access); err != nil {
			return err
		}
		_, err = eventbus.Publish(tx, eventbus.PublishInput{
			EventType:      eventbus.EventGameAccessUpserted,
			AggregateType:  "agent_game_access",
			AggregateID:    strconv.FormatUint(access.ID, 10),
			TenantID:       access.TenantID,
			BrandID:        access.BrandID,
			OccurredAt:     now,
			Producer:       "game-service",
			IdempotencyKey: "game.access.upserted:" + strconv.FormatUint(access.AgentID, 10) + ":" + strconv.FormatUint(access.GameID, 10) + ":" + string(access.Status),
			Consumers:      eventbus.ConsumersDataPlatformSync(),
			Payload:        gameAccessPayload(access),
		})
		return err
	})
	if err != nil {
		return nil, err
	}
	return &access, nil
}

func (s *Service) RevokeAccess(scope shared.Scope, agentID, gameID uint64) (model.AgentGameAccess, error) {
	access, err := s.repo.FindAgentGameAccess(agentID, gameID)
	if err != nil {
		return model.AgentGameAccess{}, err
	}
	if !shared.MatchesScopedRecord(scope, access.TenantID, access.BrandID) {
		return model.AgentGameAccess{}, gorm.ErrRecordNotFound
	}
	agent, err := s.loadAgent(s.db, access.AgentID)
	if err != nil {
		return model.AgentGameAccess{}, err
	}
	game, err := s.loadGame(s.db, access.GameID)
	if err != nil {
		return model.AgentGameAccess{}, err
	}
	if err := shared.EnsureSameScope("agent_game_access", access.TenantID, access.BrandID, agent.TenantID, agent.BrandID); err != nil {
		return model.AgentGameAccess{}, gorm.ErrRecordNotFound
	}
	if err := shared.EnsureSameScope("agent_game_access", access.TenantID, access.BrandID, game.TenantID, game.BrandID); err != nil {
		return model.AgentGameAccess{}, gorm.ErrRecordNotFound
	}
	if err := s.repo.DeleteAgentGameAccess(&access); err != nil {
		return model.AgentGameAccess{}, err
	}
	if _, err := eventbus.Publish(s.db, eventbus.PublishInput{
		EventType:      eventbus.EventGameAccessRevoked,
		AggregateType:  "agent_game_access",
		AggregateID:    strconv.FormatUint(access.ID, 10),
		TenantID:       access.TenantID,
		BrandID:        access.BrandID,
		OccurredAt:     time.Now().UTC(),
		Producer:       "game-service",
		IdempotencyKey: "game.access.revoked:" + strconv.FormatUint(access.ID, 10),
		Consumers:      eventbus.ConsumersDataPlatformSync(),
		Payload:        gameAccessRevokedPayload(access),
	}); err != nil {
		return model.AgentGameAccess{}, err
	}
	return access, nil
}

func (s *Service) loadScopedGame(scope shared.Scope, id uint64) (model.Game, error) {
	game, err := s.repo.FindGame(id)
	if err != nil {
		return model.Game{}, err
	}
	if !shared.MatchesScopedRecord(scope, game.TenantID, game.BrandID) {
		return model.Game{}, gorm.ErrRecordNotFound
	}
	return game, nil
}

func (s *Service) loadAgent(tx *gorm.DB, agentID uint64) (model.Agent, error) {
	return s.repo.FindAgentTx(tx, agentID)
}

func (s *Service) loadGame(tx *gorm.DB, gameID uint64) (model.Game, error) {
	return s.repo.FindGameTx(tx, gameID)
}

func (s *Service) ensureAgentGameAccessPayloadInScope(scope shared.Scope, input AccessInput) error {
	agent, err := s.loadAgent(s.db, input.AgentID)
	if err != nil {
		return err
	}
	game, err := s.loadGame(s.db, input.GameID)
	if err != nil {
		return err
	}
	if !shared.MatchesScopedRecord(scope, agent.TenantID, agent.BrandID) || !shared.MatchesScopedRecord(scope, game.TenantID, game.BrandID) {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Service) createDefaultGameIntegrationKey(game model.Game, createdBy string) (*model.GameIntegrationKey, string, error) {
	accessKey, err := generateToken("ga_live", 18)
	if err != nil {
		return nil, "", err
	}
	secret, err := generateToken("gsk_live", 32)
	if err != nil {
		return nil, "", err
	}
	credential := model.GameIntegrationKey{
		TenantID:         game.TenantID,
		BrandID:          game.BrandID,
		GameID:           game.ID,
		Name:             "default",
		AccessKey:        accessKey,
		SecretCiphertext: secret,
		Status:           model.GameIntegrationKeyStatusActive,
		Scopes:           jsonStringArray(defaultGameOpenAPIScopes()),
		CreatedBy:        strings.TrimSpace(createdBy),
	}
	if err := s.repo.CreateGameIntegrationKey(&credential); err != nil {
		return nil, "", err
	}
	return &credential, secret, nil
}

func (s *Service) gameCredentialView(credential model.GameIntegrationKey, secret string) *CredentialView {
	return &CredentialView{
		ID:         credential.ID,
		GameID:     credential.GameID,
		Name:       credential.Name,
		AccessKey:  credential.AccessKey,
		SecretKey:  secret,
		Status:     credential.Status,
		Scopes:     stringSliceFromJSON(credential.Scopes),
		CreatedAt:  credential.CreatedAt,
		LastUsedAt: credential.LastUsedAt,
		RotatedAt:  credential.RotatedAt,
		ExpiresAt:  credential.ExpiresAt,
		Remark:     credential.Remark,
	}
}

type deleteReferenceCheck struct {
	model   any
	query   string
	args    []any
	message string
}

func (s *Service) ensureDeleteReferencesClear(checks []deleteReferenceCheck) error {
	for _, check := range checks {
		count, err := s.repo.DeleteReferenceCount(check.model, check.query, check.args...)
		if err != nil {
			return err
		}
		if count > 0 {
			return errors.New(check.message)
		}
	}
	return nil
}

func archiveDeletedScopedCode(tx *gorm.DB, modelValue any, id uint64, column string, current string) error {
	if strings.TrimSpace(current) == "" {
		return nil
	}
	archived := current + "__deleted_" + generateArchiveSuffix(id)
	return tx.Model(modelValue).Where("id = ?", id).Update(column, archived).Error
}

func defaultGameStatus(status model.GameStatus) model.GameStatus {
	if status == "" {
		return model.GameStatusDraft
	}
	return status
}

func validateGameStatusTransition(current, next model.GameStatus) error {
	if next == "" {
		return errors.New("status is required")
	}
	if current == model.GameStatusOffline && next == model.GameStatusDraft {
		return errors.New("offline games cannot be moved back to draft")
	}
	if current == model.GameStatusArchived && next != model.GameStatusArchived {
		return errors.New("archived games cannot change status")
	}
	return nil
}

func applyGameStatus(game *model.Game, status model.GameStatus, now time.Time) {
	game.Status = status
	switch status {
	case model.GameStatusOnline:
		game.LaunchAt = &now
		game.OfflineAt = nil
	case model.GameStatusOffline:
		game.OfflineAt = &now
	case model.GameStatusDraft:
		game.LaunchAt = nil
		game.OfflineAt = nil
	}
}

func defaultGameOpenAPIScopes() []string {
	return []string{"player:create", "player:register_with_invite", "game_access:read", "rule:read", "recharge:callback"}
}

func stringSliceFromJSON(value []byte) []string {
	if len(value) == 0 {
		return nil
	}
	var items []string
	_ = jsonUnmarshal(value, &items)
	return uniqueStrings(items)
}

func jsonStringArray(values []string) []byte {
	items := uniqueStrings(values)
	if len(items) == 0 {
		return nil
	}
	payload, _ := jsonMarshal(items)
	return payload
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	return items
}

func generateToken(prefix string, byteCount int) (string, error) {
	buf := make([]byte, byteCount)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buf), nil
}

func generateArchiveSuffix(id uint64) string {
	return strings.Join([]string{
		strconv.FormatUint(id, 10),
		strconv.FormatInt(time.Now().UTC().Unix(), 10),
	}, "_")
}

func validGameOpenAPISignature(method, path, rawQuery, timestampValue, nonce string, body []byte, secret, signature string) bool {
	bodyHash := sha256.Sum256(body)
	signPayload := strings.ToUpper(method) + "\n" + path + "\n" + rawQuery + "\n" + timestampValue + "\n" + nonce + "\n" + hex.EncodeToString(bodyHash[:])
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(signPayload))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

func jsonMarshal(v any) ([]byte, error)      { return json.Marshal(v) }
func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }
