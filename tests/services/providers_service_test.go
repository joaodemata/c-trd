package services_test

import (
	"net/url"
	"testing"

	cmm "c_trd/common"
	"c_trd/providers"
	"c_trd/services"
	"c_trd/tests/testutil"
)

// buildQuoteRequest firma comun de los constructores de peticiones de precio
type buildQuoteRequest func(symbol string) (providers.HTTPRequestConfig, *cmm.ErrorHandler)

func TestBuildQuoteRequestServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		build       buildQuoteRequest
		symbol      string
		wantHost    string
		wantAuthKey string
		wantQuery   map[string]string
	}{
		{"coinmarketcap en mayusculas", services.BuildCoinMarketCapQuoteRequestService, " btc ", "pro-api.coinmarketcap.com", "X-CMC_PRO_API_KEY", map[string]string{"symbol": "BTC", "convert": "USD"}},
		{"finnhub accion", services.BuildFinnhubQuoteRequestService, "AAPL", "finnhub.io", "X-Finnhub-Token", map[string]string{"symbol": "AAPL"}},
		{"finnhub conserva el formato del simbolo", services.BuildFinnhubQuoteRequestService, " OANDA:XAU_USD ", "finnhub.io", "X-Finnhub-Token", map[string]string{"symbol": "OANDA:XAU_USD"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			request, err := tt.build(tt.symbol)
			testutil.RequireEmptyError(t, err)

			parsed, parseErr := url.Parse(request.URL)
			if parseErr != nil || parsed.Hostname() != tt.wantHost {
				t.Errorf("url = %q, want host %q", request.URL, tt.wantHost)
			}
			if request.Method != "GET" {
				t.Errorf("method = %q, want GET", request.Method)
			}
			if request.Auth == nil || request.Auth.Type != "API_KEY" || request.Auth.Key != tt.wantAuthKey || request.Auth.In != "HEADER" {
				t.Errorf("auth = %+v, want API_KEY %s in HEADER", request.Auth, tt.wantAuthKey)
			}
			if len(request.QueryData) != len(tt.wantQuery) {
				t.Errorf("query = %v, want %v", request.QueryData, tt.wantQuery)
			}
			for key, want := range tt.wantQuery {
				if request.QueryData[key] != want {
					t.Errorf("query[%s] = %q, want %q", key, request.QueryData[key], want)
				}
			}

			// La peticion debe pasar las mismas reglas que ExecuteRequest con un proveedor bien configurado
			credentials := providers.RequestCredentials{SecretRef: "PRICE_APIKEY", AllowedHosts: []string{tt.wantHost}}
			testutil.RequireEmptyError(t, providers.ValidateRequestTarget(&request, credentials))
		})
	}
}

func TestBuildQuoteRequestServices_EmptySymbol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		build        buildQuoteRequest
		symbol       string
		wantTracking string
	}{
		{"coinmarketcap sin simbolo", services.BuildCoinMarketCapQuoteRequestService, "", "SPRVE004"},
		{"coinmarketcap simbolo en blanco", services.BuildCoinMarketCapQuoteRequestService, "   ", "SPRVE004"},
		{"finnhub sin simbolo", services.BuildFinnhubQuoteRequestService, "", "SPRVE005"},
		{"finnhub simbolo en blanco", services.BuildFinnhubQuoteRequestService, "  ", "SPRVE005"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.build(tt.symbol)
			testutil.RequireTrackingCode(t, err, tt.wantTracking)
		})
	}
}
