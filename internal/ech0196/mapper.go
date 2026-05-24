package ech0196

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	"etax/internal/domain"
	"etax/internal/kursliste"
	"etax/internal/money"
)

type ClientInfo struct {
	FirstName string
	LastName  string
}

type Options struct {
	Kursliste *kursliste.Store
}

func FromReport(report *domain.Report, canton string, year int, client ClientInfo) (TaxStatement, error) {
	return FromReportWithOptions(report, canton, year, client, Options{})
}

func FromReportWithOptions(report *domain.Report, canton string, year int, client ClientInfo, options Options) (TaxStatement, error) {
	if report == nil {
		return TaxStatement{}, fmt.Errorf("report is nil")
	}
	if err := validateReportPeriod(report, year); err != nil {
		return TaxStatement{}, err
	}
	stmt := NewTaxStatement(documentID(report, year), canton, year)
	stmt.PeriodFrom = defaultString(report.PeriodFrom, stmt.PeriodFrom)
	stmt.PeriodTo = defaultString(report.PeriodTo, stmt.PeriodTo)
	stmt.Institution = Institution{Name: defaultString(report.InstitutionName, report.ProviderID), LEI: report.InstitutionLEI}
	stmt.Clients = []Client{{ClientNumber: report.AccountID, FirstName: client.FirstName, LastName: client.LastName}}
	stmt.ListOfSecurities = buildSecurities(report, options.Kursliste)
	updateTotals(&stmt)
	return stmt, nil
}

func validateReportPeriod(report *domain.Report, year int) error {
	for _, date := range []struct {
		label string
		value string
	}{
		{"periodFrom", report.PeriodFrom},
		{"periodTo", report.PeriodTo},
	} {
		if date.value == "" {
			continue
		}
		if len(date.value) < 4 || date.value[:4] != fmt.Sprintf("%04d", year) {
			return fmt.Errorf("provider report %s=%s does not match tax year %d; export the IBKR Flex statement for %d-01-01 through %d-12-31", date.label, date.value, year, year, year)
		}
	}
	return nil
}

func documentID(report *domain.Report, year int) string {
	clientNumber := leftPad(trimAlnumUpper(report.AccountID), 14)
	return fmt.Sprintf("CH%s%s%04d123101", organisationNumber(report), clientNumber, year)
}

func organisationNumber(report *domain.Report) string {
	name := defaultString(report.InstitutionName, report.ProviderID)
	if name == "" {
		return "19999"
	}
	sum := sha256.Sum256([]byte(name))
	hashPrefix := int(sum[0])<<16 | int(sum[1])<<8 | int(sum[2])
	return fmt.Sprintf("19%03d", hashPrefix%1000)
}

func buildSecurities(report *domain.Report, kurslisteStore *kursliste.Store) Securities {
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
		depot.Securities = append(depot.Securities, byISIN[isin].build(i+1, isin, kurslisteStore))
	}
	return Securities{Depots: []Depot{depot}}
}

type securityBuilder struct {
	position  *domain.Position
	trades    []domain.Trade
	cashflows []domain.CashFlow
}

type paymentEntry struct {
	key     string
	payment Payment
}

func builder(m map[string]*securityBuilder, isin string) *securityBuilder {
	if m[isin] == nil {
		m[isin] = &securityBuilder{}
	}
	return m[isin]
}

func (b *securityBuilder) build(id int, isin string, kurslisteStore *kursliste.Store) Security {
	name := isin
	currency := "CHF"
	category := "OTHER"
	if b.position != nil {
		name = truncate(defaultString(b.position.Name, b.position.Symbol, isin), 60)
		currency = defaultString(b.position.Currency, currency)
		category = categoryFor(b.position.AssetCategory)
	}
	for _, cf := range b.cashflows {
		if name == isin {
			name = truncate(defaultString(cf.Name, cf.Symbol, isin), 60)
		}
		if currency == "CHF" {
			currency = defaultString(cf.Currency, currency)
		}
	}
	sec := Security{
		PositionID:       id,
		ISIN:             validISIN(isin),
		Country:          countryFromISIN(isin),
		Currency:         currency,
		QuotationType:    "PIECE",
		NominalValue:     money.One(),
		SecurityCategory: category,
		SecurityType:     securityType(category, name),
		SecurityName:     name,
	}
	enrichSecurity(&sec, kurslisteStore, isin)
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
			Mutation:        false,
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

func enrichSecurity(sec *Security, kurslisteStore *kursliste.Store, isin string) {
	match, ok := kurslisteStore.LookupISIN(isin)
	if !ok {
		return
	}
	sec.ValorNumber = match.ValorNumber
	sec.Country = defaultString(match.Country, sec.Country)
	sec.Currency = defaultString(match.Currency, sec.Currency)
	sec.SecurityCategory = defaultString(match.SecurityGroup, sec.SecurityCategory)
	sec.SecurityType = defaultString(match.SecurityType, sec.SecurityType)
	if shouldUseKurslisteName(sec.SecurityName, isin) {
		sec.SecurityName = truncate(defaultString(match.SecurityName, sec.SecurityName), 60)
	}
	if nominalValue, err := money.FromString(match.NominalValue); match.NominalValue != "" && err == nil {
		sec.NominalValue = nominalValue
	}
}

func shouldUseKurslisteName(current, isin string) bool {
	current = strings.TrimSpace(current)
	return current == "" || strings.EqualFold(current, strings.TrimSpace(isin))
}

func paymentsFor(cashflows []domain.CashFlow, isin string) []Payment {
	var payments []paymentEntry
	for _, cf := range cashflows {
		t := strings.ToUpper(cf.Type)
		if !strings.Contains(t, "DIVIDEND") && !strings.Contains(t, "INTEREST") {
			continue
		}
		amountCHF := cf.Amount.Abs().Mul(cf.FXToCHF)
		payment := Payment{
			PaymentDate:    cf.Date,
			Name:           truncate(defaultString(cf.Description, cf.Name, cf.Symbol, isin), 200),
			QuotationType:  "PIECE",
			Quantity:       money.Zero(),
			AmountCurrency: cf.Currency,
			Amount:         cf.Amount.Abs(),
			ExchangeRate:   cf.FXToCHF,
		}
		if strings.HasPrefix(isin, "CH") {
			payment.GrossRevenueA = amountCHF
		} else {
			payment.GrossRevenueB = amountCHF
		}
		payments = append(payments, paymentEntry{
			key:     paymentMatchKey(cf),
			payment: payment,
		})
	}
	for _, cf := range cashflows {
		if !strings.Contains(strings.ToUpper(cf.Type), "WITHHOLD") {
			continue
		}
		taxCHF := cf.Amount.Neg().Mul(cf.FXToCHF)
		entry := findPaymentEntry(payments, paymentMatchKey(cf))
		if entry == nil {
			payments = append(payments, paymentEntry{
				key: paymentMatchKey(cf),
				payment: Payment{
					PaymentDate:    cf.Date,
					Name:           truncate(defaultString(cf.Description, cf.Name, cf.Symbol, isin), 200),
					QuotationType:  "PIECE",
					AmountCurrency: cf.Currency,
					ExchangeRate:   cf.FXToCHF,
				},
			})
			entry = &payments[len(payments)-1]
		}
		if strings.HasPrefix(isin, "US") {
			entry.payment.AdditionalWithHoldingUSA = entry.payment.AdditionalWithHoldingUSA.Add(taxCHF)
		} else {
			entry.payment.WithHoldingTaxClaim = entry.payment.WithHoldingTaxClaim.Add(taxCHF)
		}
	}
	sort.SliceStable(payments, func(i, j int) bool {
		if payments[i].payment.PaymentDate == payments[j].payment.PaymentDate {
			return payments[i].payment.Name < payments[j].payment.Name
		}
		return payments[i].payment.PaymentDate < payments[j].payment.PaymentDate
	})
	out := make([]Payment, 0, len(payments))
	for _, entry := range payments {
		out = append(out, entry.payment)
	}
	return out
}

func findPaymentEntry(payments []paymentEntry, key string) *paymentEntry {
	for i := range payments {
		if payments[i].key == key {
			return &payments[i]
		}
	}
	return nil
}

func paymentMatchKey(cf domain.CashFlow) string {
	return cf.Date + "\x00" + normalizedPaymentDescription(defaultString(cf.Description, cf.Name, cf.Symbol, cf.ISIN))
}

func normalizedPaymentDescription(s string) string {
	s = strings.TrimSpace(strings.ToUpper(s))
	if i := strings.Index(s, " - "); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, " ("); i >= 0 {
		s = s[:i]
	}
	return strings.Join(strings.Fields(s), " ")
}

func updateTotals(stmt *TaxStatement) {
	var taxValue, gra, grb, wht, usa money.Decimal
	for _, depot := range stmt.ListOfSecurities.Depots {
		for _, sec := range depot.Securities {
			if sec.TaxValue != nil {
				taxValue = taxValue.Add(sec.TaxValue.Value)
			}
			for _, p := range sec.Payments {
				gra = gra.Add(p.GrossRevenueA)
				grb = grb.Add(p.GrossRevenueB)
				wht = wht.Add(p.WithHoldingTaxClaim)
				usa = usa.Add(p.AdditionalWithHoldingUSA)
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
	stmt.ListOfSecurities.TotalAdditionalWHTUSA = usa
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
		if strings.Contains(name, "CONVERTIBLE") {
			return "BOND.CONVERTIBLE"
		}
		if strings.Contains(name, "OPTION") {
			return "BOND.OPTION"
		}
		return "BOND.BOND"
	case "FUND":
		return "FUND.DISTRIBUTION"
	default:
		return ""
	}
}

func mutationFor(side string) bool {
	return strings.TrimSpace(side) != ""
}

func countryFromISIN(isin string) string {
	if len(isin) >= 2 {
		return isin[:2]
	}
	return "US"
}

func validISIN(isin string) string {
	isin = strings.TrimSpace(isin)
	if len(isin) == 12 {
		return isin
	}
	return ""
}

func defaultString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func trimAlnumUpper(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(s)) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func leftPad(s string, width int) string {
	if len(s) >= width {
		return s[len(s)-width:]
	}
	return strings.Repeat("0", width-len(s)) + s
}
