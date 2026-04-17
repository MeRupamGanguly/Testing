package money

import (
	"errors"
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Money represents a monetary value with a specific currency.
type Money struct {
	amount   decimal.Decimal
	currency string
}

// NewMoney creates a new Money instance. Amount is expected in the smallest unit? No, we use decimal.
func NewMoney(amount decimal.Decimal, currency string) (Money, error) {
	if strings.TrimSpace(currency) == "" {
		return Money{}, errors.New("currency cannot be empty")
	}
	currency = strings.ToUpper(currency)
	if !IsValidCurrency(currency) {
		return Money{}, fmt.Errorf("unsupported currency: %s", currency)
	}
	return Money{amount: amount, currency: currency}, nil
}

// MustNewMoney panics on error – useful for tests or static data.
func MustNewMoney(amount decimal.Decimal, currency string) Money {
	m, err := NewMoney(amount, currency)
	if err != nil {
		panic(err)
	}
	return m
}

// MustNewMoneyFromFloat creates Money from float64. Panics on error.
func MustNewMoneyFromFloat(amount float64, currency string) Money {
	return MustNewMoney(decimal.NewFromFloat(amount), currency)
}

// Amount returns the amount as decimal.
func (m Money) Amount() decimal.Decimal {
	return m.amount
}

// Currency returns the currency code.
func (m Money) Currency() string {
	return m.currency
}

// Add returns a new Money with the sum. Currency must match.
func (m Money) Add(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("currency mismatch: %s vs %s", m.currency, other.currency)
	}
	return Money{amount: m.amount.Add(other.amount), currency: m.currency}, nil
}

// Sub subtracts other from m.
func (m Money) Sub(other Money) (Money, error) {
	if m.currency != other.currency {
		return Money{}, fmt.Errorf("currency mismatch: %s vs %s", m.currency, other.currency)
	}
	return Money{amount: m.amount.Sub(other.amount), currency: m.currency}, nil
}

// Mul multiplies the amount by a factor (decimal).
func (m Money) Mul(factor decimal.Decimal) Money {
	return Money{amount: m.amount.Mul(factor), currency: m.currency}
}

// Round returns a new Money rounded according to the currency's precision.
func (m Money) Round() Money {
	prec := GetCurrencyPrecision(m.currency)
	rounded := m.amount.Round(int32(prec))
	return Money{amount: rounded, currency: m.currency}
}

// Equals compares amount and currency.
func (m Money) Equals(other Money) bool {
	return m.currency == other.currency && m.amount.Equal(other.amount)
}

// String returns a formatted representation.
func (m Money) String() string {
	prec := int32(GetCurrencyPrecision(m.currency))
	return fmt.Sprintf("%s %s", m.currency, m.amount.StringFixed(prec))
}
