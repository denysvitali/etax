<FlexQueryResponse queryName="TaxReport" type="AF">
  <FlexStatements count="1">
    <FlexStatement accountId="U7654321" fromDate="20250101" toDate="20251231" period="LastYear" currency="CHF">
      <SecuritiesInfo>
        <SecurityInfo symbol="AAPL" isin="US0378331005" description="APPLE INC" assetCategory="STK" currency="USD"/>
      </SecuritiesInfo>
      <OpenPositions>
        <OpenPosition symbol="AAPL" isin="US0378331005" description="" assetCategory="STK" position="100" markPrice="195.00" positionValue="19500.00" currency="USD" fxRateToBase="0.88" reportDate="20251231"/>
        <OpenPosition symbol="CHSPI" isin="CH0000000001" description="" assetCategory="STK" quantity="3" markPrice="400.166666" positionValue="1200.50" currency="CHF" fxRateToBase="0.91" reportDate="20251231"/>
      </OpenPositions>
      <Trades>
        <Trade symbol="AAPL" isin="US0378331005" description="" tradeDate="20250315" buySell="buy" quantity="-50" tradePrice="170.00" proceeds="-8500.00" currency="USD" fxRateToBase="0.88"/>
        <Trade symbol="AAPL" isin="US0378331005" description="APPLE INC" tradeDate="20250820" buySell="sell" quantity="-5" tradePrice="180.00" proceeds="900.00" currency="USD" fxRateToBase="0.88"/>
      </Trades>
      <CashTransactions>
        <CashTransaction type="Dividends" symbol="AAPL" isin="US0378331005" description="AAPL CASH DIVIDEND" amount="25.00" currency="USD" fxRateToBase="0.88" dateTime="20250515;120000"/>
        <CashTransaction type="WithholdingTax" symbol="AAPL" isin="US0378331005" description="AAPL CASH DIVIDEND - US TAX" amount="-3.75" currency="USD" fxRateToBase="0.88" dateTime="20250515;120000"/>
        <CashTransaction type="Interest" symbol="" isin="" description="CHF CREDIT INTEREST" amount="1.25" currency="CHF" fxRateToBase="0.91" dateTime="20250601"/>
      </CashTransactions>
    </FlexStatement>
  </FlexStatements>
</FlexQueryResponse>
