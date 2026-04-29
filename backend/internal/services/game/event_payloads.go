package game

import "game-admin/backend/internal/domain/model"

func gameCreatedPayload(game model.Game, credentialID uint64) map[string]any {
	return map[string]any{
		"id":           game.ID,
		"gameCode":     game.GameCode,
		"name":         game.Name,
		"vendor":       game.Vendor,
		"category":     game.Category,
		"status":       game.Status,
		"isAgentable":  game.IsAgentable,
		"credentialID": credentialID,
	}
}

func gameUpdatedPayload(before, after model.Game) map[string]any {
	return map[string]any{
		"id":             after.ID,
		"gameCode":       after.GameCode,
		"name":           after.Name,
		"beforeStatus":   before.Status,
		"status":         after.Status,
		"isAgentable":    after.IsAgentable,
		"beforeGameCode": before.GameCode,
	}
}

func gameStatusChangedPayload(before, after model.Game) map[string]any {
	return map[string]any{
		"id":           after.ID,
		"gameCode":     after.GameCode,
		"beforeStatus": before.Status,
		"status":       after.Status,
	}
}

func gameDeletedPayload(game model.Game) map[string]any {
	return map[string]any{
		"id":          game.ID,
		"gameCode":    game.GameCode,
		"name":        game.Name,
		"status":      game.Status,
		"isAgentable": game.IsAgentable,
	}
}

func gameCredentialRotatedPayload(credential model.GameIntegrationKey) map[string]any {
	return map[string]any{
		"id":        credential.ID,
		"gameID":    credential.GameID,
		"accessKey": credential.AccessKey,
		"status":    credential.Status,
	}
}

func gameAccessPayload(access model.AgentGameAccess) map[string]any {
	return map[string]any{
		"id":        access.ID,
		"agentID":   access.AgentID,
		"gameID":    access.GameID,
		"status":    access.Status,
		"grantedBy": access.GrantedBy,
	}
}

func gameAccessRevokedPayload(access model.AgentGameAccess) map[string]any {
	return map[string]any{
		"id":      access.ID,
		"agentID": access.AgentID,
		"gameID":  access.GameID,
		"status":  access.Status,
	}
}
