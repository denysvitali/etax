package ech0196

import (
	"encoding/xml"
	"fmt"
	"time"

	"etax/internal/money"
)

const (
	NamespaceECH0196 = "http://www.ech.ch/xmlns/eCH-0196/2"
	NamespaceXSI     = "http://www.w3.org/2001/XMLSchema-instance"
	SchemaLocation   = "http://www.ech.ch/xmlns/eCH-0196/2 http://www.ech.ch/xmlns/eCH-0196/2/eCH-0196-2-2.xsd"
	MinorVersion     = 22
)

type TaxStatement struct {
	XMLName                  xml.Name      `xml:"taxStatement"`
	XMLNS                    string        `xml:"xmlns,attr"`
	XMLNSXSI                 string        `xml:"xmlns:xsi,attr,omitempty"`
	XSISchemaLocation        string        `xml:"xsi:schemaLocation,attr,omitempty"`
	ID                       string        `xml:"id,attr"`
	MinorVersion             int           `xml:"minorVersion,attr"`
	CreationDate             string        `xml:"creationDate,attr"`
	TaxPeriod                int           `xml:"taxPeriod,attr"`
	PeriodFrom               string        `xml:"periodFrom,attr"`
	PeriodTo                 string        `xml:"periodTo,attr"`
	Country                  string        `xml:"country,attr"`
	Canton                   string        `xml:"canton,attr"`
	TotalTaxValue            money.Decimal `xml:"totalTaxValue,attr"`
	TotalGrossRevenueA       money.Decimal `xml:"totalGrossRevenueA,attr"`
	TotalGrossRevenueB       money.Decimal `xml:"totalGrossRevenueB,attr"`
	TotalWithHoldingTaxClaim money.Decimal `xml:"totalWithHoldingTaxClaim,attr"`
	Institution              Institution   `xml:"institution"`
	Clients                  []Client      `xml:"client"`
	ListOfSecurities         Securities    `xml:"listOfSecurities"`
}

type Institution struct {
	LEI  string `xml:"lei,attr,omitempty"`
	Name string `xml:"name,attr"`
}

type Client struct {
	ClientNumber string `xml:"clientNumber,attr"`
	FirstName    string `xml:"firstName,attr,omitempty"`
	LastName     string `xml:"lastName,attr,omitempty"`
}

type Securities struct {
	TotalTaxValue            money.Decimal `xml:"totalTaxValue,attr"`
	TotalGrossRevenueA       money.Decimal `xml:"totalGrossRevenueA,attr"`
	TotalGrossRevenueB       money.Decimal `xml:"totalGrossRevenueB,attr"`
	TotalWithHoldingTaxClaim money.Decimal `xml:"totalWithHoldingTaxClaim,attr"`
	TotalLumpSumTaxCredit    money.Decimal `xml:"totalLumpSumTaxCredit,attr"`
	TotalNonRecoverableTax   money.Decimal `xml:"totalNonRecoverableTax,attr"`
	TotalAdditionalWHTUSA    money.Decimal `xml:"totalAdditionalWithHoldingTaxUSA,attr"`
	TotalGrossRevenueIUP     money.Decimal `xml:"totalGrossRevenueIUP,attr"`
	TotalGrossRevenueConv    money.Decimal `xml:"totalGrossRevenueConversion,attr"`
	Depots                   []Depot       `xml:"depot"`
}

type Depot struct {
	DepotNumber string     `xml:"depotNumber,attr"`
	Securities  []Security `xml:"security"`
}

type Security struct {
	PositionID       int           `xml:"positionId,attr"`
	ValorNumber      string        `xml:"valorNumber,attr,omitempty"`
	ISIN             string        `xml:"isin,attr,omitempty"`
	Country          string        `xml:"country,attr"`
	Currency         string        `xml:"currency,attr"`
	QuotationType    string        `xml:"quotationType,attr"`
	NominalValue     money.Decimal `xml:"nominalValue,attr"`
	SecurityCategory string        `xml:"securityCategory,attr"`
	SecurityType     string        `xml:"securityType,attr,omitempty"`
	SecurityName     string        `xml:"securityName,attr"`
	TaxValue         *TaxValue     `xml:"taxValue,omitempty"`
	Payments         []Payment     `xml:"payment,omitempty"`
	Stocks           []Stock       `xml:"stock,omitempty"`
}

type TaxValue struct {
	ReferenceDate   string        `xml:"referenceDate,attr"`
	QuotationType   string        `xml:"quotationType,attr"`
	Quantity        money.Decimal `xml:"quantity,attr"`
	BalanceCurrency string        `xml:"balanceCurrency,attr"`
	UnitPrice       money.Decimal `xml:"unitPrice,attr"`
	Value           money.Decimal `xml:"value,attr"`
	ExchangeRate    money.Decimal `xml:"exchangeRate,attr"`
}

type Payment struct {
	PaymentDate              string        `xml:"paymentDate,attr"`
	Name                     string        `xml:"name,attr,omitempty"`
	QuotationType            string        `xml:"quotationType,attr"`
	Quantity                 money.Decimal `xml:"quantity,attr"`
	AmountCurrency           string        `xml:"amountCurrency,attr"`
	Amount                   money.Decimal `xml:"amount,attr"`
	ExchangeRate             money.Decimal `xml:"exchangeRate,attr"`
	GrossRevenueA            money.Decimal `xml:"grossRevenueA,attr"`
	GrossRevenueB            money.Decimal `xml:"grossRevenueB,attr"`
	WithHoldingTaxClaim      money.Decimal `xml:"withHoldingTaxClaim,attr"`
	AdditionalWithHoldingUSA money.Decimal `xml:"additionalWithHoldingTaxUSA,attr,omitempty"`
}

type Stock struct {
	ReferenceDate   string        `xml:"referenceDate,attr"`
	Mutation        bool          `xml:"mutation,attr"`
	Name            string        `xml:"name,attr,omitempty"`
	QuotationType   string        `xml:"quotationType,attr"`
	Quantity        money.Decimal `xml:"quantity,attr"`
	BalanceCurrency string        `xml:"balanceCurrency,attr"`
	UnitPrice       money.Decimal `xml:"unitPrice,attr"`
	Balance         money.Decimal `xml:"balance,attr"`
	Value           money.Decimal `xml:"value,attr"`
	ExchangeRate    money.Decimal `xml:"exchangeRate,attr"`
}

func NewTaxStatement(id, canton string, taxPeriod int) TaxStatement {
	return TaxStatement{
		XMLNS:             NamespaceECH0196,
		XMLNSXSI:          NamespaceXSI,
		XSISchemaLocation: SchemaLocation,
		ID:                id,
		MinorVersion:      MinorVersion,
		CreationDate:      time.Now().UTC().Format(time.RFC3339),
		TaxPeriod:         taxPeriod,
		PeriodFrom:        dateFor(taxPeriod, "01-01"),
		PeriodTo:          dateFor(taxPeriod, "12-31"),
		Country:           "CH",
		Canton:            canton,
	}
}

func dateFor(year int, suffix string) string {
	return fmt.Sprintf("%04d-%s", year, suffix)
}
