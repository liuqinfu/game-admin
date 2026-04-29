package agent

import "game-admin/backend/internal/domain/model"

func playerBindingPayload(player model.Player, binding *model.Binding) map[string]any {
	return map[string]any{
		"player":  player,
		"binding": binding,
	}
}
