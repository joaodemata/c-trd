package services

import (
	"c_trd/models"
	"c_trd/providers"
	"context"
	"fmt"
	"net/http"
	"strings"

	cmm "c_trd/common"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Llamadas explicitas a las APIs de los proveedores. La autenticacion sale del secret_ref del proveedor
// y el host debe estar en sus allowed_hosts
// TODO: esto posiblemente sea mejor usar el idApi y hacer un query por si cambia la url en el futuro
// TODO: y realmente lo mejor para obtener el precio es tener un websocket apuntando a la coleccion assets y agregar un campo price del activo (asumiendo siempre dolares)
const (
	coinMarketCapQuotesURL     = "https://pro-api.coinmarketcap.com/v2/cryptocurrency/quotes/latest"
	coinMarketCapQuoteCurrency = "USD"
	finnhubQuoteURL            = "https://finnhub.io/api/v1/quote"
)

// Un reintento ante 429 o 5xx, el hook tiene pocos segundos para responder
var providerPriceRetryPolicy = providers.RetryPolicy{MaxRetries: 1, DelayMs: 1000}

// GetAssetMarketPriceService consulta el precio actual del asset en el proveedor de su configs (price_provider y
// price_symbol). El ctx limita la peticion HTTP; las consultas a la base de datos usan su propio timeout
func GetAssetMarketPriceService(ctx context.Context, idAsset primitive.ObjectID) (float64, *cmm.ErrorHandler) {
	config, err := GetAssetPriceConfigService(idAsset)
	if !err.IsEmtpy() {
		return 0, err
	}

	if config.PriceProvider == "" || config.PriceSymbol == "" {
		return 0, cmm.NewErrorHandler(fmt.Errorf("asset %s", idAsset.Hex()), "El asset no existe o no tiene configs.price_provider y configs.price_symbol", cmm.LevelError, "SPRVE001")
	}

	provider, err := GetProviderByTagService(config.PriceProvider)
	if !err.IsEmtpy() {
		return 0, err
	}

	if provider.ID.IsZero() {
		return 0, cmm.NewErrorHandler(fmt.Errorf("provider %q", config.PriceProvider), "El proveedor del precio del asset no existe", cmm.LevelError, "SPRVE002")
	}

	var price float64
	switch config.PriceProvider {
	case cmm.PriceProviderCoinMarketCap:
		price, err = GetCoinMarketCapQuotePriceService(ctx, provider, config.PriceSymbol)
	case cmm.PriceProviderFinnhub:
		price, err = GetFinnhubQuotePriceService(ctx, provider, config.PriceSymbol)
	default:
		err = cmm.NewErrorHandler(fmt.Errorf("price_provider %q", config.PriceProvider), "El price_provider del asset no esta soportado", cmm.LevelError, "SPRVE003")
	}
	if !err.IsEmtpy() {
		return 0, err
	}

	return price, cmm.NewEmptyErrorHandler()
}

// --- CoinMarketCap ---

// BuildCoinMarketCapQuoteRequestService peticion del precio en USD del simbolo en /v2/cryptocurrency/quotes/latest
func BuildCoinMarketCapQuoteRequestService(symbol string) (providers.HTTPRequestConfig, *cmm.ErrorHandler) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return providers.HTTPRequestConfig{}, cmm.NewErrorHandler(nil, "El asset no tiene price_symbol", cmm.LevelError, "SPRVE004")
	}

	retryPolicy := providerPriceRetryPolicy

	request := providers.HTTPRequestConfig{
		URL:          coinMarketCapQuotesURL,
		Method:       http.MethodGet,
		DataLocation: cmm.ApiDataLocationQuery,
		QueryData:    map[string]string{"symbol": symbol, "convert": coinMarketCapQuoteCurrency},
		Auth:         &providers.HTTPAuth{Type: "API_KEY", Key: "X-CMC_PRO_API_KEY", In: "HEADER"},
		RetryPolicy:  &retryPolicy,
	}

	return request, cmm.NewEmptyErrorHandler()
}

// GetCoinMarketCapQuotePriceService consulta el precio actual en USD de la criptomoneda en CoinMarketCap
func GetCoinMarketCapQuotePriceService(ctx context.Context, provider models.ProvidersModelType, symbol string) (float64, *cmm.ErrorHandler) {
	request, err := BuildCoinMarketCapQuoteRequestService(symbol)
	if !err.IsEmtpy() {
		return 0, err
	}

	credentials := providers.RequestCredentials{SecretRef: provider.SecretRef, AllowedHosts: provider.AllowedHosts}
	responseBody, err := providers.ExecuteRequest(ctx, &request, credentials)
	if !err.IsEmtpy() {
		return 0, err
	}

	price, err := providers.CoinMarketCapQuotePrice(responseBody, symbol)
	if !err.IsEmtpy() {
		return 0, err
	}

	return price, cmm.NewEmptyErrorHandler()
}

// --- Finnhub ---

// BuildFinnhubQuoteRequestService peticion del precio actual del simbolo en /api/v1/quote. El simbolo se envia tal
// cual porque Finnhub distingue formatos (ej: AAPL, ^GSPC u OANDA:XAU_USD)
func BuildFinnhubQuoteRequestService(symbol string) (providers.HTTPRequestConfig, *cmm.ErrorHandler) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return providers.HTTPRequestConfig{}, cmm.NewErrorHandler(nil, "El asset no tiene price_symbol", cmm.LevelError, "SPRVE005")
	}

	retryPolicy := providerPriceRetryPolicy

	request := providers.HTTPRequestConfig{
		URL:          finnhubQuoteURL,
		Method:       http.MethodGet,
		DataLocation: cmm.ApiDataLocationQuery,
		QueryData:    map[string]string{"symbol": symbol},
		Auth:         &providers.HTTPAuth{Type: "API_KEY", Key: "X-Finnhub-Token", In: "HEADER"},
		RetryPolicy:  &retryPolicy,
	}

	return request, cmm.NewEmptyErrorHandler()
}

// GetFinnhubQuotePriceService consulta el precio actual del simbolo en Finnhub
func GetFinnhubQuotePriceService(ctx context.Context, provider models.ProvidersModelType, symbol string) (float64, *cmm.ErrorHandler) {
	request, err := BuildFinnhubQuoteRequestService(symbol)
	if !err.IsEmtpy() {
		return 0, err
	}

	credentials := providers.RequestCredentials{SecretRef: provider.SecretRef, AllowedHosts: provider.AllowedHosts}
	responseBody, err := providers.ExecuteRequest(ctx, &request, credentials)
	if !err.IsEmtpy() {
		return 0, err
	}

	price, err := providers.FinnhubQuotePrice(responseBody)
	if !err.IsEmtpy() {
		return 0, err
	}

	return price, cmm.NewEmptyErrorHandler()
}
