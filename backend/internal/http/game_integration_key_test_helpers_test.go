package http

func defaultGameOpenAPIScopes() []string {
	return []string{"player:create", "player:register_with_invite", "game_access:read", "rule:read", "recharge:callback"}
}
