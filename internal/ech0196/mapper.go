package ech0196

import (
	"fmt"
	"sort"
	"strings"

	"etax/internal/domain"
	"etax/internal/money"
)

type ClientInfo struct {
	FirstName string
	LastName  string
}

func FromReport(report *domain.Report, canton string, year int, client ClientInfo) (TaxStatement, error) {
	if report == nil {
		return TaxStatement{}, fmt.Errorf("report is nil")
	}
	stmt := NewTaxStatement(fmt.Sprintf("%s-%s-%d", report.ProviderID, report.AccountID, year), canton, year)
	stmt.PeriodFrom = defaultString(report.PeriodFrom, stmt.PeriodFrom)
	stmt.PeriodTo = defaultString(report.PeriodTo, stmt.PeriodTo)
	stmt.Institution = Institution{Name: defaultString(report.InstitutionName, report.ProviderID), LEI: report.InstitutionLEI}
	stmt.Clients = []Client{{ClientNumber: report.AccountID, FirstName: client.FirstName, LastName: client.LastName}}
	stmt.ListOfSecurities = buildSecurities(report)
	updateTotals(&stmt)
	return stmt, nil
}

func buildSecurities(report *domain.Report) Securities {
	byISIN := map[string]*securityBuilder{}
	for _, p := range report.Positions {
		if p.ISIN == "" {
			continue
		}
		builder(byISIN, p.ISIN).position = &p
	}
	for _, t := range report.Trades {
		if t.ISIN == "" {
			continue
		}
		builder(byISIN, t.ISIN).trades = append(builder(byISIN, t.ISIN).trades, t)
	}
	for _, cf := range report.CashFlows {
		if cf.ISIN == "" {
			continue
		}
		builder(byISIN, cf.ISIN).cashflows = append(builder(byISIN, cf.ISIN).cashflows, cf)
	}

	keys := make([]string, 0, len(byISIN))
	for k := range byISIN {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	depot := Depot{DepotNumber: report.AccountID}
	for i, isin := range keys {
		depot.Securities = append(depot.Securities, byISIN[isin].build(i+1, isin))
	}
	return Securities{Depots: []Depot{depot}}
}

type securityBuilder struct {
	position  *domain.Position
	trades    []domain.Trade
	cashflows []domain.CashFlow
}

func builder(m map[string]*securityBuilder, isin string) *securityBuilder {
	if m[isin] == nil {
		m[isin] = &securityBuilder{}
	}
	return m[isin]
}

func (b *securityBuilder) build(id int, isin string) Security {
	name := isin
	currency := "CHF"
	category := "OTHER"
	if b.position != nil {
		name = defaultString(b.position.Name, b.position.Symbol, isin)
		currency = defaultString(b.position.Currency, currency)
		category = categoryFor(b.position.AssetCategory)
	}
	for _, cf := range b.cashflows {
		if name == isin {
			name = defaultString(cf.Name, cf.Symbol, isin)
		}
		if currency == "CHF" {
			currency = defaultString(cf.Currency, currency)
		}
	}
	sec := Security{
		PositionID:       id,
		ISIN:             isin,
		Country:          countryFromISIN(isin),
		Currency:         currency,
		QuotationType:    "PIECE",
		NominalValue:     money.One(),
		SecurityCategory: category,
		SecurityType:     securityType(category, name),
		SecurityName:     name,
	}
	if b.position != nil {
		p := b.position
		sec.TaxValue = &TaxValue{
			ReferenceDate:   defaultString(p.ReferenceDate, "0001-01-01"),
			QuotationType:   "PIECE",
			Quantity:        p.Quantity,
			BalanceCurrency: p.Currency,
			UnitPrice:       p.UnitPrice,
			Value:           p.Value,
			ExchangeRate:    p.FXToCHF,
		}
		sec.Stocks = append(sec.Stocks, Stock{
			ReferenceDate:   defaultString(p.ReferenceDate, "0001-01-01"),
			Mutation:        0,
			Name:            "Year-end balance",
			QuotationType:   "PIECE",
			Quantity:        p.Quantity,
			BalanceCurrency: p.Currency,
			UnitPrice:       p.UnitPrice,
			Balance:         p.Quantity,
			Value:           p.Value,
			ExchangeRate:    p.FXToCHF,
		})
	}
	for _, t := range b.trades {
		sec.Stocks = append(sec.Stocks, Stock{
			ReferenceDate:   t.Date,
			Mutation:        mutationFor(t.Side),
			Name:            t.Side,
			QuotationType:   "PIECE",
			Quantity:        t.Quantity,
			BalanceCurrency: t.Currency,
			UnitPrice:       t.Price,
			Balance:         t.Quantity,
			Value:           t.Value,
			ExchangeRate:    t.FXToCHF,
		})
	}
	sec.Payments = paymentsFor(b.cashflows, isin)
	return sec
}

func paymentsFor(cashflows []domain.CashFlow, isin string) []Payment {
	type key struct{ date, name string }
	payments := map[key]*Payment{}
	for _, cf := range cashflows {
		t := strings.ToUpper(cf.Type)
		if !strings.Contains(t, "DIVIDEND") && !strings.Contains(t, "INTEREST") {
			continue
		}
		amountCHF := cf.Amount.Abs().Mul(cf.FXToCHF)
		k := key{date: cf.Date, name: defaultString(cf.Name, cf.Symbol, isin)}
		payments[k] = &Payment{
			PaymentDate:    cf.Date,
			Name:           k.name,
			Quantity:       money.Zero(),
			AmountCurrency: cf.Currency,
			Amount:         cf.Amount.Abs(),
			ExchangeRate:   cf.FXToCHF,
		}
		if strings.HasPrefix(isin, "CH") {
			payments[k].GrossRevenueA = amountCHF
		} else {
			payments[k].GrossRevenueB = amountCHF
		}
	}
	for _, cf := range cashflows {
		if !strings.Contains(strings.ToUpper(cf.Type), "WITHHOLD") {
			continue
		}
		k := key{date: cf.Date, name: defaultString(cf.Name, cf.Symbol, isin)}
		if payments[k] == nil {
			payments[k] = &Payment{PaymentDate: cf.Date, Name: k.name, AmountCurrency: cf.Currency, ExchangeRate: cf.FXToCHF}
		}
		taxCHF := cf.Amount.Abs().Mul(cf.FXToCHF)
		if strings.HasPrefix(isin, "US") {
			payments[k].AdditionalWithHoldingUSA = taxCHF
		} else {
			payments[k].WithHoldingTaxClaim = taxCHF
		}
	}
	keys := make([]key, 0, len(payments))
	for k := range payments {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].date == keys[j].date {
			return keys[i].name < keys[j].name
		}
		return keys[i].date < keys[j].date
	})
	out := make([]Payment, 0, len(keys))
	for _, k := range keys {
		out = append(out, *payments[k])
	}
	return out
}

func updateTotals(stmt *TaxStatement) {
	var taxValue, gra, grb, wht money.Decimal
	for _, depot := range stmt.ListOfSecurities.Depots {
		for _, sec := range depot.Securities {
			if sec.TaxValue != nil {
				taxValue = taxValue.Add(sec.TaxValue.Value)
			}
			for _, p := range sec.Payments {
				gra = gra.Add(p.GrossRevenueA)
				grb = grb.Add(p.GrossRevenueB)
				wht = wht.Add(p.WithHoldingTaxClaim)
			}
		}
	}
	stmt.TotalTaxValue = taxValue
	stmt.TotalGrossRevenueA = gra
	stmt.TotalGrossRevenueB = grb
	stmt.TotalWithHoldingTaxClaim = wht
	stmt.ListOfSecurities.TotalTaxValue = taxValue
	stmt.ListOfSecurities.TotalGrossRevenueA = gra
	stmt.ListOfSecurities.TotalGrossRevenueB = grb
	stmt.ListOfSecurities.TotalWithHoldingTaxClaim = wht
}

func categoryFor(asset string) string {
	switch strings.ToUpper(asset) {
	case "STK":
		return "SHARE"
	case "BOND":
		return "BOND"
	case "FUND":
		return "FUND"
	case "OPT", "FOP":
		return "OPTION"
	case "CASH":
		return "CURRNOTE"
	default:
		return "OTHER"
	}
}

func securityType(category, name string) string {
	name = strings.ToUpper(name)
	switch category {
	case "SHARE":
		if strings.Contains(name, "PREFERRED") || strings.Contains(name, "PREF") {
			return "SHARE.PREFERRED"
		}
		return "SHARE.COMMON"
	case "BOND":
		return "BOND.STRAIGHT"
	case "FUND":
		return "FUND.DISTRIBUTION"
	default:
		return ""
	}
}

func mutationFor(side string) int {
	switch strings.ToUpper(side) {
	case "BUY":
		return 1
	case "SELL":
		return 2
	default:
		return 0
	}
}

func countryFromISIN(isin string) string {
	if len(isin) >= 2 {
		return isin[:2]
	}
	return "US"
}

func defaultString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
