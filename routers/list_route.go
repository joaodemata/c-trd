package routers

import (
	"net/http"

	cmm "c_trd/common"
	c "c_trd/controllers"
	dv "c_trd/data_validates"
	fv "c_trd/format_validates"
)

func listRoutes(server *http.ServeMux) *http.ServeMux {

	// Router Decorator pointing to server
	api := cmm.NewRouter(server)

	// Routes
	// Sesion opcional: sin sesion solo las oportunidades publicas; con sesion tambien las del usuario y las de lista blanca donde esta
	api.Get("/list/opportunities", c.ListOportunitiesController(), generalRateLimit, cmm.OptionalAuthenticateRequestMiddleware, dv.OptionalAuthDataValidate, cmm.PaginationFormatValidate)
	// Creacion de condiciones: se listan las APIs y luego los callbacks del response_format de la API elegida
	api.Get("/list/apis", c.ListApisController(), generalRateLimit, cmm.PaginationFormatValidate)
	api.Get("/list/callbacks", c.ListCallbacksController(), generalRateLimit, fv.ListCallbacksFormatValidate)
	// Selectores del frontend para crear triggers y subir el CSV de Google Trends
	api.Get("/list/assets", c.ListAssetsController(), generalRateLimit, cmm.PaginationFormatValidate)
	api.Get("/list/catalogs", c.ListCatalogsController(), generalRateLimit, fv.ListCatalogsFormatValidate)
	// Sesion opcional: sin sesion o sin privilegio solo las condiciones activas o inactivas; tagStatus solo el super usuario
	api.Get("/list/conditions", c.ListConditionsController(), generalRateLimit, cmm.OptionalAuthenticateRequestMiddleware, dv.OptionalAuthDataValidate, cmm.PaginationFormatValidate, fv.ListConditionsFormatValidate, dv.ListConditionsDataValidate)
	api.Get("/list/triggers", c.ListTriggersController(), generalRateLimit, cmm.PaginationFormatValidate, fv.ListTriggersFormatValidate, dv.ListTriggersDataValidate)
	// Agentes: cola de tesis (FIFO) y corridas para que el super usuario las revise
	api.Get("/list/agent_theses", c.ListAgentThesesController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), cmm.PaginationFormatValidate, fv.ListAgentThesesFormatValidate)
	api.Get("/list/agent_runs", c.ListAgentRunsController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), cmm.PaginationFormatValidate, fv.ListAgentRunsFormatValidate)
	// Portfolios del usuario de la sesion y las transacciones de uno de ellos (startDate y endDate por fecha de ejecucion)
	api.Get("/list/portfolios", c.ListPortfoliosController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), cmm.PaginationFormatValidate)
	api.Get("/list/portfolio_transactions", c.ListPortfolioTransactionsController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), cmm.PaginationFormatValidate, fv.ListPortfolioTransactionsFormatValidate, dv.ListPortfolioTransactionsDataValidate)
	// ROI calculado al momento con los precios de mercado (una consulta por asset), por eso va aparte del listado de portfolios
	api.Get("/list/portfolio_roi", c.ListPortfolioRoiController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(portfolioPrivileges...), fv.PortfolioRoiFormatValidate, dv.PortfolioRoiDataValidate)

	// Backtesting (super usuario): datasets, corridas sin operaciones y el detalle de una corrida con sus operaciones
	api.Get("/list/backtest_datasets", c.ListBacktestDatasetsController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), cmm.PaginationFormatValidate, fv.ListBacktestDatasetsFormatValidate)
	api.Get("/list/backtest_runs", c.ListBacktestRunsController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), cmm.PaginationFormatValidate, fv.ListBacktestRunsFormatValidate)
	api.Get("/list/backtest_run", c.GetBacktestRunController(), generalRateLimit, cmm.AuthenticateRequestMiddleware, dv.AuthDataValidate, dv.PrivilegesDataValidate(cmm.UserPrivilegeSuperUser.Tag), fv.GetBacktestRunFormatValidate, dv.GetBacktestRunDataValidate)

	return server

}
