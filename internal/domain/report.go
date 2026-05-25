package domain

import (
	"context"
	"io"

	"github.com/denysvitali/etax/internal/money"
)

type Provider interface {
	ID() string
	Name() string
	Parse(ctx context.Context, r io.Reader) (*Report, error)
}

type Report struct {
	ProviderID      string
	AccountID       string
	BaseCurrency    string
	PeriodFrom      string
	PeriodTo        string
	InstitutionName string
	InstitutionLEI  string
	Positions       []Position
	Trades          []Trade
	CashFlows       []CashFlow
}

// ISINs returns unique non-empty ISINs in first-seen report order.
func (r *Report) ISINs() []string {
	if r == nil {
		return nil
	}

	seen := map[string]bool{}
	var isins []string
	add := func(isin string) {
		if isin == "" || seen[isin] {
			return
		}
		seen[isin] = true
		isins = append(isins, isin)
	}

	for _, position := range r.Positions {
		add(position.ISIN)
	}
	for _, trade := range r.Trades {
		add(trade.ISIN)
	}
	for _, cashflow := range r.CashFlows {
		add(cashflow.ISIN)
	}
	return isins
}

type Position struct {
	Symbol        string
	ISIN          string
	Name          string
	AssetCategory string
	Quantity      money.Decimal
	UnitPrice     money.Decimal
	Value         money.Decimal
	Currency      string
	FXToCHF       money.Decimal
	ReferenceDate string
}

type Trade struct {
	Symbol   string
	ISIN     string
	Name     string
	Date     string
	Side     string
	Quantity money.Decimal
	Price    money.Decimal
	Value    money.Decimal
	Currency string
	FXToCHF  money.Decimal
}

type CashFlow struct {
	Type        string
	Symbol      string
	ISIN        string
	Name        string
	Description string
	Amount      money.Decimal
	Currency    string
	FXToCHF     money.Decimal
	Date        string
}
