package money

// CurrencyConfig holds rounding precision and other metadata.
type CurrencyConfig struct {
	Code      string
	Precision uint8 // number of decimal places
	Symbol    string
}

var currencies = map[string]CurrencyConfig{
	"USD": {Code: "USD", Precision: 2, Symbol: "$"},
	"EUR": {Code: "EUR", Precision: 2, Symbol: "€"},
	"GBP": {Code: "GBP", Precision: 2, Symbol: "£"},
	"JPY": {Code: "JPY", Precision: 0, Symbol: "¥"},
}

// RegisterCurrency allows adding or overriding a currency.
func RegisterCurrency(cfg CurrencyConfig) {
	currencies[cfg.Code] = cfg
}

// IsValidCurrency checks if a currency code is supported.
func IsValidCurrency(code string) bool {
	_, ok := currencies[code]
	return ok
}

// GetCurrencyPrecision returns the number of decimal places for a currency.
func GetCurrencyPrecision(code string) uint8 {
	if cfg, ok := currencies[code]; ok {
		return cfg.Precision
	}
	return 2 // fallback
}
