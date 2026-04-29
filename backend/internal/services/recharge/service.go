package recharge

import (
	"errors"

	"game-admin/backend/internal/domain/model"
	"game-admin/backend/internal/services/shared"
	"gorm.io/gorm"
)

type Service struct {
	db   *gorm.DB
	repo Repository
}

func NewService(db *gorm.DB) *Service { return &Service{db: db, repo: newRepository(db)} }

func (s *Service) ValidateCallbackScope(scope shared.Scope, gameID uint64) (model.Game, error) {
	if gameID == 0 {
		return model.Game{}, errors.New("gameID is required")
	}
	game, err := s.repo.FindGame(gameID)
	if err != nil {
		return model.Game{}, err
	}
	if !shared.MatchesScopedRecord(scope, game.TenantID, game.BrandID) {
		return model.Game{}, gorm.ErrRecordNotFound
	}
	return game, nil
}
