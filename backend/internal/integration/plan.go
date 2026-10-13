package integration

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	EventSaleFinalized      = "SALE_FINALIZED_V1"
	EventPurchaseReceipt    = "PURCHASE_RECEIPT_V1"
	EventSalesReturnRestock = "SALES_RETURN_RESTOCK_V1"
	EventPaymentVerified    = "PAYMENT_VERIFIED_V1"
	EventPaymentRefunded    = "PAYMENT_REFUNDED_V1"
)

type Envelope struct {
	ExternalEventID string          `json:"external_event_id"`
	EventType       string          `json:"event_type"`
	PayloadVersion  int             `json:"payload_version"`
	OccurredAt      string          `json:"occurred_at"`
	AccountingDate  string          `json:"accounting_date"`
	Payload         json.RawMessage `json:"payload"`
}

type SaleRevenueLine struct {
	SKU        string `json:"sku"`
	Quantity   string `json:"quantity"`
	RevenueMMK int64  `json:"revenue_mmk"`
	TaxMMK     int64  `json:"tax_mmk"`
}

type SettlementLine struct {
	RuleCode  string `json:"rule_code"`
	Direction string `json:"direction"`
	AmountMMK int64  `json:"amount_mmk"`
}

type CostLine struct {
	SKU               string `json:"sku"`
	Quantity          string `json:"quantity"`
	HistoricalCostMMK int64  `json:"historical_cost_mmk"`
	ValueMMK          int64  `json:"value_mmk"`
}

type SalePayload struct {
	OrderPublicID         string            `json:"order_public_id"`
	AccountingDate        string            `json:"accounting_date"`
	SourceChannel         string            `json:"source_channel"`
	CustomerSegment       string            `json:"customer_segment"`
	Currency              string            `json:"currency"`
	RevenueLines          []SaleRevenueLine `json:"revenue_lines"`
	DeliveryFeeMMK        int64             `json:"delivery_fee_mmk"`
	SettlementAdjustments []SettlementLine  `json:"settlement_adjustments"`
	NetReceivableMMK      int64             `json:"net_receivable_mmk"`
	DepositAppliedMMK     int64             `json:"deposit_applied_mmk"`
	InventoryCostLines    []CostLine        `json:"inventory_cost_lines"`
}

type PurchasePayload struct {
	PurchaseReceiptPublicID string     `json:"purchase_receipt_public_id"`
	PurchaseOrderPublicID   string     `json:"purchase_order_public_id"`
	SupplierName            string     `json:"supplier_name"`
	AccountingDate          string     `json:"accounting_date"`
	Currency                string     `json:"currency"`
	Lines                   []CostLine `json:"lines"`
}

type ReturnPayload struct {
	ReturnPublicID string     `json:"return_public_id"`
	OrderPublicID  string     `json:"order_public_id"`
	AccountingDate string     `json:"accounting_date"`
	Currency       string     `json:"currency"`
	Lines          []CostLine `json:"lines"`
}

type PaymentPayload struct {
	PaymentPublicID  string `json:"payment_public_id"`
	OrderPublicID    string `json:"order_public_id"`
	AccountingDate   string `json:"accounting_date"`
	Method           string `json:"method"`
	AmountMMK        int64  `json:"amount_mmk"`
	Currency         string `json:"currency"`
	SettlementTarget string `json:"settlement_target"`
	SaleFinalized    bool   `json:"sale_finalized"`
}

type PlannedLine struct {
	SourceKey string
	Memo      string
	Debit     int64
	Credit    int64
}

type Plan struct {
	Date            string
	Description     string
	Currency        string
	SalesChannelKey string
	CustomerSegment string
	Lines           []PlannedLine
	Total           int64
}

type Target struct {
	Kind               string
	AccountID          string
	AccountType        string
	Postable           bool
	Active             bool
	FinancialAccountID string
	Currency           string
	SalesChannelID     string
}

type Resolver func(sourceKey string) (Target, error)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func Validation(message string) *Error {
	return &Error{Status: 422, Code: "validation_failed", Message: message}
}

func Mapping(key string) *Error {
	return &Error{Status: 422, Code: "mapping_required", Message: "missing mapping " + key}
}

func Conflict(message string) *Error {
	return &Error{Status: 409, Code: "payload_conflict", Message: message}
}

var structuralKeys = []string{
	"AR",
	"INVENTORY_ASSET",
	"PURCHASE_RECEIPT_CREDIT",
	"TAX_PAYABLE",
	"DELIVERY_REVENUE",
	"CUSTOMER_DEPOSITS",
	"SALES_CHANNEL:MESSENGER",
	"SALES_CHANNEL:WEBSITE",
	"SALES_CHANNEL:PHONE",
	"SALES_CHANNEL:WALK_IN",
	"SALES_CHANNEL:ADMIN",
	"SALES_CHANNEL:OTHER",
	"PAYMENT_METHOD:CASH",
	"PAYMENT_METHOD:KBZPAY",
	"PAYMENT_METHOD:BANK_TRANSFER",
}

func StructuralKeys() []string {
	out := make([]string, len(structuralKeys))
	copy(out, structuralKeys)
	return out
}

func ParseEnvelope(raw []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return Envelope{}, Validation("request body must be JSON")
	}
	env.EventType = strings.TrimSpace(env.EventType)
	env.ExternalEventID = strings.TrimSpace(env.ExternalEventID)
	env.AccountingDate = strings.TrimSpace(env.AccountingDate)
	if env.ExternalEventID == "" || env.EventType == "" || env.AccountingDate == "" {
		return Envelope{}, Validation("external_event_id, event_type, and accounting_date are required")
	}
	if env.PayloadVersion == 0 {
		env.PayloadVersion = 1
	}
	if len(env.Payload) == 0 {
		return Envelope{}, Validation("payload is required")
	}
	return env, nil
}

func PlanEvent(env Envelope, resolve Resolver) (Plan, error) {
	switch env.EventType {
	case EventSaleFinalized:
		return planSale(env, resolve)
	case EventPurchaseReceipt:
		return planPurchase(env, resolve)
	case EventSalesReturnRestock:
		return planReturn(env, resolve)
	case EventPaymentVerified:
		return planPayment(env, false, resolve)
	case EventPaymentRefunded:
		return planPayment(env, true, resolve)
	default:
		return Plan{}, Validation("unsupported event type")
	}
}

func planSale(env Envelope, resolve Resolver) (Plan, error) {
	var p SalePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return Plan{}, Validation("sale payload is invalid")
	}
	if strings.TrimSpace(p.OrderPublicID) == "" {
		return Plan{}, Validation("order_public_id is required")
	}
	currency, err := currencyOf(p.Currency)
	if err != nil {
		return Plan{}, err
	}
	segment := strings.ToUpper(strings.TrimSpace(p.CustomerSegment))
	switch segment {
	case "CONSUMER", "RETAILER", "DISTRIBUTOR", "OTHER":
	default:
		return Plan{}, Validation("customer_segment must be CONSUMER, RETAILER, DISTRIBUTOR, or OTHER")
	}
	channel := strings.ToUpper(strings.TrimSpace(p.SourceChannel))
	if channel == "" {
		return Plan{}, Validation("source_channel is required")
	}
	var lines []PlannedLine
	var revenue, tax, additions, deductions int64
	for _, line := range p.RevenueLines {
		if line.RevenueMMK < 0 || line.TaxMMK < 0 {
			return Plan{}, Validation("sale amounts cannot be negative")
		}
		sku := strings.ToUpper(strings.TrimSpace(line.SKU))
		if line.RevenueMMK > 0 {
			if sku == "" {
				return Plan{}, Validation("revenue line sku is required")
			}
			key := "REVENUE:" + sku
			if _, err := need(resolve, key); err != nil {
				return Plan{}, err
			}
			lines = append(lines, PlannedLine{SourceKey: key, Credit: line.RevenueMMK, Memo: sku + " revenue"})
			revenue += line.RevenueMMK
		}
		tax += line.TaxMMK
	}
	if tax > 0 {
		if _, err := need(resolve, "TAX_PAYABLE"); err != nil {
			return Plan{}, err
		}
		lines = append(lines, PlannedLine{SourceKey: "TAX_PAYABLE", Credit: tax, Memo: "tax"})
	}
	if p.DeliveryFeeMMK < 0 || p.NetReceivableMMK < 0 || p.DepositAppliedMMK < 0 {
		return Plan{}, Validation("sale amounts cannot be negative")
	}
	if p.DeliveryFeeMMK > 0 {
		if _, err := need(resolve, "DELIVERY_REVENUE"); err != nil {
			return Plan{}, err
		}
		lines = append(lines, PlannedLine{SourceKey: "DELIVERY_REVENUE", Credit: p.DeliveryFeeMMK, Memo: "delivery"})
	}
	for _, adj := range p.SettlementAdjustments {
		if adj.AmountMMK < 0 {
			return Plan{}, Validation("settlement amounts cannot be negative")
		}
		if adj.AmountMMK == 0 {
			continue
		}
		code := strings.ToUpper(strings.TrimSpace(adj.RuleCode))
		if code == "" {
			return Plan{}, Validation("settlement rule_code is required")
		}
		dir := strings.ToUpper(strings.TrimSpace(adj.Direction))
		switch dir {
		case "DEDUCTION":
			key := "SETTLEMENT_DEDUCTION:" + code
			if _, err := need(resolve, key); err != nil {
				return Plan{}, err
			}
			lines = append(lines, PlannedLine{SourceKey: key, Debit: adj.AmountMMK, Memo: code})
			deductions += adj.AmountMMK
		case "ADDITION":
			key := "SETTLEMENT_ADDITION:" + code
			if _, err := need(resolve, key); err != nil {
				return Plan{}, err
			}
			lines = append(lines, PlannedLine{SourceKey: key, Credit: adj.AmountMMK, Memo: code})
			additions += adj.AmountMMK
		default:
			return Plan{}, Validation("settlement direction must be DEDUCTION or ADDITION")
		}
	}
	receivable := p.NetReceivableMMK + p.DeliveryFeeMMK
	if receivable+deductions != revenue+tax+p.DeliveryFeeMMK+additions {
		return Plan{}, Validation("sale payload does not balance")
	}
	if p.DepositAppliedMMK > receivable {
		return Plan{}, Validation("deposit applied exceeds the sale receivable")
	}
	ar := receivable - p.DepositAppliedMMK
	if ar > 0 {
		if _, err := need(resolve, "AR"); err != nil {
			return Plan{}, err
		}
		lines = append(lines, PlannedLine{SourceKey: "AR", Debit: ar, Memo: "accounts receivable"})
	}
	if p.DepositAppliedMMK > 0 {
		if _, err := need(resolve, "CUSTOMER_DEPOSITS"); err != nil {
			return Plan{}, err
		}
		lines = append(lines, PlannedLine{SourceKey: "CUSTOMER_DEPOSITS", Debit: p.DepositAppliedMMK, Memo: "customer deposit applied"})
	}
	var cost int64
	for _, line := range p.InventoryCostLines {
		if line.HistoricalCostMMK < 0 {
			return Plan{}, Validation("inventory cost cannot be negative")
		}
		if line.HistoricalCostMMK == 0 {
			continue
		}
		sku := strings.ToUpper(strings.TrimSpace(line.SKU))
		if sku == "" {
			return Plan{}, Validation("inventory cost sku is required")
		}
		key := "COGS:" + sku
		if _, err := need(resolve, key); err != nil {
			return Plan{}, err
		}
		if _, err := need(resolve, "INVENTORY_ASSET"); err != nil {
			return Plan{}, err
		}
		lines = append(lines, PlannedLine{SourceKey: key, Debit: line.HistoricalCostMMK, Memo: sku + " cost"})
		lines = append(lines, PlannedLine{SourceKey: "INVENTORY_ASSET", Credit: line.HistoricalCostMMK, Memo: sku + " inventory"})
		cost += line.HistoricalCostMMK
	}
	_ = cost
	plan := Plan{
		Date:            env.AccountingDate,
		Description:     "MyanKafe sale " + p.OrderPublicID,
		Currency:        currency,
		SalesChannelKey: "SALES_CHANNEL:" + channel,
		CustomerSegment: segment,
		Lines:           lines,
	}
	if _, err := need(resolve, plan.SalesChannelKey); err != nil {
		return Plan{}, err
	}
	return finish(plan)
}

func planPurchase(env Envelope, resolve Resolver) (Plan, error) {
	var p PurchasePayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return Plan{}, Validation("purchase receipt payload is invalid")
	}
	currency, err := currencyOf(p.Currency)
	if err != nil {
		return Plan{}, err
	}
	var lines []PlannedLine
	var total int64
	for _, line := range p.Lines {
		value := line.ValueMMK
		if value == 0 {
			value = line.HistoricalCostMMK
		}
		if value < 0 {
			return Plan{}, Validation("receipt value cannot be negative")
		}
		if value == 0 {
			continue
		}
		total += value
	}
	if total <= 0 {
		return Plan{}, Validation("purchase receipt has no value")
	}
	if _, err := need(resolve, "INVENTORY_ASSET"); err != nil {
		return Plan{}, err
	}
	if _, err := need(resolve, "PURCHASE_RECEIPT_CREDIT"); err != nil {
		return Plan{}, err
	}
	lines = append(lines, PlannedLine{SourceKey: "INVENTORY_ASSET", Debit: total, Memo: "inventory receipt"})
	lines = append(lines, PlannedLine{SourceKey: "PURCHASE_RECEIPT_CREDIT", Credit: total, Memo: "purchase receipt credit"})
	return finish(Plan{
		Date:        env.AccountingDate,
		Description: "MyanKafe purchase receipt " + strings.TrimSpace(p.PurchaseReceiptPublicID),
		Currency:    currency,
		Lines:       lines,
	})
}

func planReturn(env Envelope, resolve Resolver) (Plan, error) {
	var p ReturnPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return Plan{}, Validation("sales return payload is invalid")
	}
	currency, err := currencyOf(p.Currency)
	if err != nil {
		return Plan{}, err
	}
	var lines []PlannedLine
	for _, line := range p.Lines {
		value := line.HistoricalCostMMK
		if value == 0 {
			value = line.ValueMMK
		}
		if value < 0 {
			return Plan{}, Validation("return value cannot be negative")
		}
		if value == 0 {
			continue
		}
		sku := strings.ToUpper(strings.TrimSpace(line.SKU))
		if sku == "" {
			return Plan{}, Validation("return sku is required")
		}
		key := "COGS:" + sku
		if _, err := need(resolve, key); err != nil {
			return Plan{}, err
		}
		if _, err := need(resolve, "INVENTORY_ASSET"); err != nil {
			return Plan{}, err
		}
		lines = append(lines, PlannedLine{SourceKey: "INVENTORY_ASSET", Debit: value, Memo: sku + " restock"})
		lines = append(lines, PlannedLine{SourceKey: key, Credit: value, Memo: sku + " cost reversal"})
	}
	if len(lines) == 0 {
		return Plan{}, Validation("restock return has no historical value")
	}
	return finish(Plan{
		Date:        env.AccountingDate,
		Description: "MyanKafe sales return " + strings.TrimSpace(p.ReturnPublicID),
		Currency:    currency,
		Lines:       lines,
	})
}

func planPayment(env Envelope, refund bool, resolve Resolver) (Plan, error) {
	var p PaymentPayload
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return Plan{}, Validation("payment payload is invalid")
	}
	if p.AmountMMK <= 0 {
		return Plan{}, Validation("payment amount must be positive")
	}
	currency, err := currencyOf(p.Currency)
	if err != nil {
		return Plan{}, err
	}
	method := strings.ToUpper(strings.TrimSpace(p.Method))
	if method == "" {
		return Plan{}, Validation("payment method is required")
	}
	target := strings.ToUpper(strings.TrimSpace(p.SettlementTarget))
	if target != "AR" && target != "CUSTOMER_DEPOSITS" {
		return Plan{}, Validation("settlement_target must be AR or CUSTOMER_DEPOSITS")
	}
	payKey := "PAYMENT_METHOD:" + method
	if _, err := need(resolve, payKey); err != nil {
		return Plan{}, err
	}
	if _, err := need(resolve, target); err != nil {
		return Plan{}, err
	}
	desc := "MyanKafe payment " + strings.TrimSpace(p.PaymentPublicID)
	var lines []PlannedLine
	if refund {
		desc = "MyanKafe refund " + strings.TrimSpace(p.PaymentPublicID)
		lines = []PlannedLine{
			{SourceKey: target, Debit: p.AmountMMK, Memo: "refund settlement"},
			{SourceKey: payKey, Credit: p.AmountMMK, Memo: "refund tender"},
		}
	} else {
		lines = []PlannedLine{
			{SourceKey: payKey, Debit: p.AmountMMK, Memo: "payment tender"},
			{SourceKey: target, Credit: p.AmountMMK, Memo: "payment settlement"},
		}
	}
	return finish(Plan{Date: env.AccountingDate, Description: desc, Currency: currency, Lines: lines})
}

func finish(plan Plan) (Plan, error) {
	var debit, credit int64
	for _, line := range plan.Lines {
		if line.Debit < 0 || line.Credit < 0 || (line.Debit > 0 && line.Credit > 0) || (line.Debit == 0 && line.Credit == 0) {
			return Plan{}, Validation("journal line is empty or two-sided")
		}
		debit += line.Debit
		credit += line.Credit
	}
	if debit == 0 || debit != credit {
		return Plan{}, Validation("journal does not balance")
	}
	plan.Total = debit
	if len(plan.Date) != 10 {
		return Plan{}, Validation("accounting_date must be YYYY-MM-DD")
	}
	return plan, nil
}

func need(resolve Resolver, key string) (Target, error) {
	target, err := resolve(key)
	if err != nil {
		return Target{}, err
	}
	if err := AcceptTarget(key, target); err != nil {
		return Target{}, err
	}
	return target, nil
}

func currencyOf(raw string) (string, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if raw == "" {
		raw = "MMK"
	}
	if raw != "MMK" {
		return "", Validation("only MMK integration events are accepted")
	}
	return raw, nil
}

func AcceptTarget(sourceKey string, target Target) error {
	if !target.Active {
		return Validation(sourceKey + " target is inactive")
	}
	kind := MappingKind(sourceKey)
	switch kind {
	case "FINANCIAL_ACCOUNT":
		if target.Kind != "FINANCIAL_ACCOUNT" || target.FinancialAccountID == "" || target.AccountID == "" {
			return Validation(sourceKey + " must map to a financial account")
		}
		if target.Currency != "MMK" {
			return Validation(sourceKey + " must map to an MMK financial account")
		}
		if !target.Postable {
			return Validation(sourceKey + " financial account ledger is not postable")
		}
	case "SALES_CHANNEL":
		if target.Kind != "SALES_CHANNEL" || target.SalesChannelID == "" {
			return Validation(sourceKey + " must map to a sales channel")
		}
	default:
		if target.Kind != "ACCOUNT" || target.AccountID == "" || !target.Postable {
			return Validation(sourceKey + " must map to an active postable account")
		}
		if !accountTypeAllowed(sourceKey, target.AccountType) {
			return Validation(sourceKey + " is mapped to the wrong account type")
		}
	}
	return nil
}

func MappingKind(sourceKey string) string {
	switch {
	case strings.HasPrefix(sourceKey, "PAYMENT_METHOD:"):
		return "FINANCIAL_ACCOUNT"
	case strings.HasPrefix(sourceKey, "SALES_CHANNEL:"):
		return "SALES_CHANNEL"
	default:
		return "ACCOUNT"
	}
}

func accountTypeAllowed(sourceKey, accountType string) bool {
	switch {
	case sourceKey == "AR" || sourceKey == "INVENTORY_ASSET":
		return accountType == "ASSET"
	case sourceKey == "TAX_PAYABLE" || sourceKey == "CUSTOMER_DEPOSITS":
		return accountType == "LIABILITY"
	case sourceKey == "PURCHASE_RECEIPT_CREDIT":
		return accountType == "LIABILITY" || accountType == "EQUITY"
	case sourceKey == "DELIVERY_REVENUE" || strings.HasPrefix(sourceKey, "REVENUE:") || strings.HasPrefix(sourceKey, "SETTLEMENT_ADDITION:"):
		return accountType == "INCOME"
	case strings.HasPrefix(sourceKey, "COGS:"):
		return accountType == "EXPENSE"
	case strings.HasPrefix(sourceKey, "SETTLEMENT_DEDUCTION:"):
		return accountType == "EXPENSE" || accountType == "INCOME"
	default:
		return false
	}
}

func ValidSourceKey(sourceKey string) bool {
	sourceKey = strings.TrimSpace(sourceKey)
	switch sourceKey {
	case "AR", "INVENTORY_ASSET", "CUSTOMER_DEPOSITS", "PURCHASE_RECEIPT_CREDIT", "TAX_PAYABLE", "DELIVERY_REVENUE":
		return true
	}
	prefix, rest, ok := strings.Cut(sourceKey, ":")
	if !ok || rest == "" || strings.Contains(rest, ":") {
		return false
	}
	switch prefix {
	case "REVENUE", "COGS", "SETTLEMENT_DEDUCTION", "SETTLEMENT_ADDITION", "PAYMENT_METHOD", "SALES_CHANNEL":
		return validKeyRest(rest)
	default:
		return false
	}
}

func validKeyRest(rest string) bool {
	if len(rest) == 0 || len(rest) > 80 {
		return false
	}
	for _, r := range rest {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func Amount(n int64) string {
	return fmt.Sprintf("%d.000000", n)
}
