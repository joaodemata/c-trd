package routers

import (
	"net/http"

	cmm "c_trd/common"
	"c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)

func operationRoutes(server *http.ServeMux) *http.ServeMux {
	// Router Decorator pointing to server
	api := cmm.NewRouter(server)
	// Routes
	api.Post("/operation/create/oportunity", controllers.CreateOportunityController(), generalRateLimit)
	// Sesion -> privilegio -> format validates (trigger y visibilidad del mismo body) -> data validates
	// El agente tambien crea triggers y condiciones: nacen pendientes de revision del super usuario
	api.Post("/operation/create/trigger", controllers.CreateTriggerController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag, cmm.UserPrivilegeAgent.Tag), fv.CreateTriggerFormatValidate, fv.VisibilityFormatValidate, dv.VisibilityDataValidate, dv.CreateTriggerDataValidate)
	// Reemplaza el trigger: el anterior queda borrado e inactivo y el nuevo apunta a el (id_reference e id_reference_root)
	api.Put("/operation/update/trigger", controllers.UpdateTriggerController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.UpdateTriggerFormatValidate, dv.UpdateTriggerDataValidate)
	api.Post("/operation/create/condition", controllers.CreateConditionController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag, cmm.UserPrivilegeAgent.Tag), fv.CreateConditionFormatValidate, fv.VisibilityFormatValidate, dv.VisibilityDataValidate, dv.CreateConditionDataValidate)

	// Revision del super usuario de lo que crean los agentes. Rechazar borra logicamente; aprobar un trigger aprueba sus condiciones
	api.Put("/operation/review/condition", controllers.ReviewConditionController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.ReviewConditionFormatValidate, dv.ReviewConditionDataValidate)
	api.Put("/operation/review/trigger", controllers.ReviewTriggerController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.ReviewTriggerFormatValidate, dv.ReviewTriggerDataValidate)

	// Cola de tesis de los agentes (super usuario)
	api.Post("/operation/create/agent_thesis", controllers.CreateAgentThesisController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.CreateAgentThesisFormatValidate, dv.CreateAgentThesisDataValidate)
	api.Put("/operation/delete/agent_thesis", controllers.DeleteAgentThesisController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.DeleteAgentThesisFormatValidate, dv.DeleteAgentThesisDataValidate)
	// Corridas del runner de los jueves en la noche (usuario agente): toma hasta 5 tesis FIFO y cierra cada corrida
	api.Post("/operation/agent/start_run", controllers.StartAgentRunController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeAgent.Tag))
	api.Put("/operation/agent/finish_run", controllers.FinishAgentRunController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeAgent.Tag), fv.FinishAgentRunFormatValidate, dv.FinishAgentRunDataValidate)

	// CSV de Google Trends por asset: se sube manualmente y las condiciones lo leen desde el mismo servidor
	api.Post("/operation/google_trends/upload", controllers.GoogleTrendsUploadController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.GoogleTrendsUploadFormatValidate, dv.GoogleTrendsUploadDataValidate)
	api.Post("/operation/google_trends/series", controllers.GoogleTrendsSeriesController(), cmm.LocalOnlyMiddleware, fv.GoogleTrendsSeriesFormatValidate, dv.GoogleTrendsSeriesDataValidate)

	// Portfolio del usuario (super usuario o generico): cada uno solo ve y edita los suyos. Borrar el portfolio borra sus transacciones
	api.Post("/operation/create/portfolio", controllers.CreatePortfolioController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), fv.CreatePortfolioFormatValidate, dv.CreatePortfolioDataValidate)
	api.Put("/operation/update/portfolio", controllers.UpdatePortfolioController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), fv.UpdatePortfolioFormatValidate, dv.UpdatePortfolioDataValidate)
	api.Put("/operation/delete/portfolio", controllers.DeletePortfolioController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), fv.DeletePortfolioFormatValidate, dv.DeletePortfolioDataValidate)
	// Transacciones: cada cambio reconstruye la posicion del asset en el portfolio desde su historial
	api.Post("/operation/create/portfolio_transaction", controllers.CreatePortfolioTransactionController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), fv.CreatePortfolioTransactionFormatValidate, dv.CreatePortfolioTransactionDataValidate)
	api.Put("/operation/update/portfolio_transaction", controllers.UpdatePortfolioTransactionController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), fv.UpdatePortfolioTransactionFormatValidate, dv.UpdatePortfolioTransactionDataValidate)
	api.Put("/operation/delete/portfolio_transaction", controllers.DeletePortfolioTransactionController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), fv.DeletePortfolioTransactionFormatValidate, dv.DeletePortfolioTransactionDataValidate)

	// Backtesting (super usuario): el CSV "Export chart data" de TradingView es uno por asset y temporalidad y trae las
	// señales de varios triggers; la subida tiene tiempos de conexion mas largos. Las corridas quedan en cola y las ejecuta el binario cmd/backtester, nunca el servidor
	api.Post("/operation/backtest/upload_dataset", controllers.BacktestDatasetUploadController(), generalRateLimit, cmm.RequestDeadlineMiddleware(cmm.BacktestUploadReadTimeout, cmm.BacktestUploadWriteTimeout), cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.BacktestDatasetUploadFormatValidate, dv.BacktestDatasetUploadDataValidate)
	api.Post("/operation/backtest/create_runs", controllers.CreateBacktestRunsController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.CreateBacktestRunsFormatValidate, dv.CreateBacktestRunsDataValidate)

	return server

}
