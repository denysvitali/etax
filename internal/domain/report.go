package domain

import (
	"context"
	"io"

	"github.com/etax-converter/etax/internal/money"
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
