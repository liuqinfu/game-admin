package http

import "github.com/gin-gonic/gin"

func registerFinanceRoutes(engine *gin.Engine, api *gin.RouterGroup, deps RouterDependencies, cfg RouterConfig) {
	registerFinanceOrderRoutes(engine, api, deps, cfg)
	registerFinanceWithdrawalRoutes(engine, api, deps, cfg)
	registerFinanceSettlementRoutes(engine, api, deps, cfg)
}
