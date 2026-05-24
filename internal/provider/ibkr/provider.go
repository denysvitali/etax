package ibkr

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"etax/internal/domain"
	"etax/internal/money"
)

const (
	institutionName = "Interactive Brokers"
	institutionLEI  = "5493004J90J71E0E4R31"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) ID() string   { return "ibkr" }
func (p *Provider) Name() string { return "Interactive Brokers Flex Query" }

func (p *Provider) Parse(_ context.Context, r io.Reader) (*domain.Report, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading IBKR XML: %w", err)
	}

	var root flexQueryResponse
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = true
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("parsing IBKR XML: %w", err)
	}
	if len(root.FlexStatements.Statements) == 0 {
		return nil, fmt.Errorf("IBKR XML does not contain a FlexStatement")
	}

	stmt := root.FlexStatements.Statements[0]
	secInfo := make(map[string]securityInfo)
	for _, s := range stmt.SecuritiesInfo.Items {
		if s.ISIN != "" {
			secInfo[s.ISIN] = s
		}
	}

	report := &domain.Report{
		ProviderID:      p.ID(),
		AccountID:       stmt.AccountID,
		BaseCurrency:    defaultString(stmt.Currency, "CHF"),
		PeriodFrom:      ibkrDate(stmt.FromDate),
		PeriodTo:        ibkrDate(stmt.ToDate),
		InstitutionName: institutionName,
		InstitutionLEI:  institutionLEI,
	}

	for _, pos := range stmt.OpenPositions.Positions {
		info := secInfo[pos.ISIN]
		report.Positions = append(report.Positions, domain.Position{
			Symbol:        firstNonEmpty(pos.Symbol, info.Symbol),
			ISIN:          pos.ISIN,
			Name:          firstNonEmpty(pos.Description, info.Description, pos.Symbol),
			AssetCategory: firstNonEmpty(pos.AssetCategory, info.AssetCategory),
			Quantity:      pos.Quantity,
			UnitPrice:     pos.MarkPrice,
			Value:         toCHF(pos.Value, pos.Currency, pos.FXToBase),
			Currency:      defaultString(pos.Currency, info.Currency),
			FXToCHF:       fx(pos.Currency, pos.FXToBase),
			ReferenceDate: ibkrDate(firstNonEmpty(pos.ReportDate, stmt.ToDate)),
		})
	}

	for _, tr := range stmt.Trades.Trades {
		info := secInfo[tr.ISIN]
		report.Trades = append(report.Trades, domain.Trade{
			Symbol:   firstNonEmpty(tr.Symbol, info.Symbol),
			ISIN:     tr.ISIN,
			Name:     firstNonEmpty(tr.Description, info.Description, tr.Symbol),
			Date:     ibkrDate(tr.Date),
			Side:     strings.ToUpper(tr.BuySell),
			Quantity: tr.Quantity.Abs(),
			Price:    tr.Price,
			Value:    toCHF(tr.Proceeds.Abs(), tr.Currency, tr.FXToBase),
			Currency: tr.Currency,
			FXToCHF:  fx(tr.Currency, tr.FXToBase),
		})
	}

	for _, tx := range stmt.CashTransactions.Transactions {
		info := secInfo[tx.ISIN]
		report.CashFlows = append(report.CashFlows, domain.CashFlow{
			Type:        tx.Type,
			Symbol:      firstNonEmpty(tx.Symbol, info.Symbol),
			ISIN:        tx.ISIN,
			Name:        firstNonEmpty(info.Description, tx.Symbol),
			Description: tx.Description,
			Amount:      tx.Amount,
			Currency:    tx.Currency,
			FXToCHF:     fx(tx.Currency, tx.FXToBase),
			Date:        ibkrDate(tx.DateTime),
		})
	}

	return report, nil
}

func toCHF(amount money.Decimal, currency string, rate money.Decimal) money.Decimal {
	if strings.EqualFold(currency, "CHF") || rate.IsZero() {
		return amount
	}
	return amount.Mul(rate)
}

func fx(currency string, rate money.Decimal) money.Decimal {
	if strings.EqualFold(currency, "CHF") || rate.IsZero() {
		return money.One()
	}
	return rate
}

func ibkrDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 8 {
		return s[:4] + "-" + s[4:6] + "-" + s[6:8]
	}
	return s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func defaultString(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
