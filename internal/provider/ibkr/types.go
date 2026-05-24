package ibkr

import (
	"encoding/xml"

	"github.com/denysvitali/etax/internal/money"
)

type flexQueryResponse struct {
	XMLName        xml.Name       `xml:"FlexQueryResponse"`
	QueryName      string         `xml:"queryName,attr"`
	Type           string         `xml:"type,attr"`
	FlexStatements flexStatements `xml:"FlexStatements"`
}

type flexStatements struct {
	Count      int             `xml:"count,attr"`
	Statements []flexStatement `xml:"FlexStatement"`
}

type flexStatement struct {
	AccountID        string           `xml:"accountId,attr"`
	FromDate         string           `xml:"fromDate,attr"`
	ToDate           string           `xml:"toDate,attr"`
	Period           string           `xml:"period,attr"`
	Currency         string           `xml:"currency,attr"`
	OpenPositions    openPositions    `xml:"OpenPositions"`
	Trades           trades           `xml:"Trades"`
	CashTransactions cashTransactions `xml:"CashTransactions"`
	SecuritiesInfo   securitiesInfo   `xml:"SecuritiesInfo"`
}

type securitiesInfo struct {
	Items []securityInfo `xml:"SecurityInfo"`
}

type securityInfo struct {
	Symbol        string `xml:"symbol,attr"`
	ISIN          string `xml:"isin,attr"`
	Description   string `xml:"description,attr"`
	AssetCategory string `xml:"assetCategory,attr"`
	Currency      string `xml:"currency,attr"`
}

type openPositions struct {
	Positions []openPosition `xml:"OpenPosition"`
}

type openPosition struct {
	Symbol        string        `xml:"symbol,attr"`
	ISIN          string        `xml:"isin,attr"`
	Description   string        `xml:"description,attr"`
	AssetCategory string        `xml:"assetCategory,attr"`
	Quantity      money.Decimal `xml:"quantity,attr"`
	Position      money.Decimal `xml:"position,attr"`
	CostBasis     money.Decimal `xml:"costBasisMoney,attr"`
	CostPrice     money.Decimal `xml:"costBasisPrice,attr"`
	MarkPrice     money.Decimal `xml:"markPrice,attr"`
	Value         money.Decimal `xml:"positionValue,attr"`
	Currency      string        `xml:"currency,attr"`
	FXToBase      money.Decimal `xml:"fxRateToBase,attr"`
	ReportDate    string        `xml:"reportDate,attr"`
}

type trades struct {
	Trades []trade `xml:"Trade"`
}

type trade struct {
	Symbol      string        `xml:"symbol,attr"`
	ISIN        string        `xml:"isin,attr"`
	Description string        `xml:"description,attr"`
	AssetCat    string        `xml:"assetCategory,attr"`
	Date        string        `xml:"tradeDate,attr"`
	BuySell     string        `xml:"buySell,attr"`
	Quantity    money.Decimal `xml:"quantity,attr"`
	Price       money.Decimal `xml:"tradePrice,attr"`
	Proceeds    money.Decimal `xml:"proceeds,attr"`
	Currency    string        `xml:"currency,attr"`
	FXToBase    money.Decimal `xml:"fxRateToBase,attr"`
}

type cashTransactions struct {
	Transactions []cashTransaction `xml:"CashTransaction"`
}

type cashTransaction struct {
	Type        string        `xml:"type,attr"`
	Symbol      string        `xml:"symbol,attr"`
	ISIN        string        `xml:"isin,attr"`
	Description string        `xml:"description,attr"`
	Amount      money.Decimal `xml:"amount,attr"`
	Currency    string        `xml:"currency,attr"`
	FXToBase    money.Decimal `xml:"fxRateToBase,attr"`
	DateTime    string        `xml:"dateTime,attr"`
}
