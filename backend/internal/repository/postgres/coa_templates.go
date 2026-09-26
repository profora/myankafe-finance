package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/profora/myankafe-finance/backend/internal/ids"
)

const (
	TemplateMyanKafeBusiness = "MYANKAFE_BUSINESS"
	TemplateRoyalMasterpiece = "ROYAL_MASTERPIECE"
	TemplatePersonal         = "PERSONAL"
	TemplateGeneralBusiness  = "GENERAL_BUSINESS"

	SystemRoleOpeningBalanceEquity     = "OPENING_BALANCE_EQUITY"
	SystemRoleOpeningBalanceAdjustment = "OPENING_BALANCE_ADJUSTMENT"
)

type chartAccount struct {
	Code, Name, Type, Parent, SystemRole string
	Postable                             bool
}

func chartTemplate(name string) ([]chartAccount, error) {
	switch name {
	case TemplateGeneralBusiness:
		return businessChart(nil), nil
	case TemplateMyanKafeBusiness:
		return businessChart(myanKafeAccounts()), nil
	case TemplateRoyalMasterpiece:
		return businessChart(royalMasterpieceAccounts()), nil
	case TemplatePersonal:
		return personalChart(), nil
	default:
		return nil, fmt.Errorf("unknown chart of accounts template")
	}
}

func businessChart(extra []chartAccount) []chartAccount {
	base := []chartAccount{
		{Code: "1000", Name: "Assets", Type: "ASSET"},
		{Code: "1100", Name: "Cash & Cash Equivalents", Type: "ASSET", Parent: "1000"},
		{Code: "1110", Name: "Cash on Hand", Type: "ASSET", Parent: "1100", Postable: true},
		{Code: "1120", Name: "Bank Accounts", Type: "ASSET", Parent: "1100"},
		{Code: "1130", Name: "Mobile Wallets", Type: "ASSET", Parent: "1100"},
		{Code: "1200", Name: "Receivables", Type: "ASSET", Parent: "1000"},
		{Code: "1210", Name: "Accounts Receivable", Type: "ASSET", Parent: "1200", Postable: true},
		{Code: "1250", Name: "Due from Related Entities", Type: "ASSET", Parent: "1000"},
		{Code: "1300", Name: "Inventory & Supplies", Type: "ASSET", Parent: "1000"},
		{Code: "1310", Name: "Inventory", Type: "ASSET", Parent: "1300", Postable: true},
		{Code: "1320", Name: "Operating Supplies", Type: "ASSET", Parent: "1300", Postable: true},
		{Code: "1400", Name: "Prepayments & Deposits", Type: "ASSET", Parent: "1000"},
		{Code: "1410", Name: "Prepaid Expenses", Type: "ASSET", Parent: "1400", Postable: true},
		{Code: "1420", Name: "Deposits", Type: "ASSET", Parent: "1400", Postable: true},
		{Code: "1500", Name: "Property & Equipment", Type: "ASSET", Parent: "1000"},
		{Code: "1510", Name: "Equipment", Type: "ASSET", Parent: "1500", Postable: true},
		{Code: "1520", Name: "Furniture & Fixtures", Type: "ASSET", Parent: "1500", Postable: true},
		{Code: "1530", Name: "Vehicles", Type: "ASSET", Parent: "1500", Postable: true},
		{Code: "1590", Name: "Accumulated Depreciation", Type: "ASSET", Parent: "1500", Postable: true},
		{Code: "2000", Name: "Liabilities", Type: "LIABILITY"},
		{Code: "2100", Name: "Payables & Accruals", Type: "LIABILITY", Parent: "2000"},
		{Code: "2110", Name: "Accounts Payable", Type: "LIABILITY", Parent: "2100", Postable: true},
		{Code: "2120", Name: "Accrued Expenses", Type: "LIABILITY", Parent: "2100", Postable: true},
		{Code: "2130", Name: "Taxes Payable", Type: "LIABILITY", Parent: "2100", Postable: true},
		{Code: "2140", Name: "Payroll Payable", Type: "LIABILITY", Parent: "2100", Postable: true},
		{Code: "2150", Name: "Customer Deposits / Unearned Revenue", Type: "LIABILITY", Parent: "2100", Postable: true},
		{Code: "2200", Name: "Financial Liabilities", Type: "LIABILITY", Parent: "2000"},
		{Code: "2210", Name: "Credit Card Payable", Type: "LIABILITY", Parent: "2200", Postable: true},
		{Code: "2220", Name: "Short-term Loans", Type: "LIABILITY", Parent: "2200", Postable: true},
		{Code: "2250", Name: "Due to Related Entities", Type: "LIABILITY", Parent: "2000"},
		{Code: "2300", Name: "Long-term Liabilities", Type: "LIABILITY", Parent: "2000"},
		{Code: "2310", Name: "Long-term Loans", Type: "LIABILITY", Parent: "2300", Postable: true},
		{Code: "3000", Name: "Equity", Type: "EQUITY"},
		{Code: "3100", Name: "Owner's Capital", Type: "EQUITY", Parent: "3000", Postable: true},
		{Code: "3200", Name: "Owner Drawings / Distributions", Type: "EQUITY", Parent: "3000", Postable: true},
		{Code: "3300", Name: "Retained Earnings", Type: "EQUITY", Parent: "3000", Postable: true},
		{Code: "3980", Name: "Opening Balance Adjustment", Type: "EQUITY", Parent: "3000", Postable: true, SystemRole: SystemRoleOpeningBalanceAdjustment},
		{Code: "3990", Name: "Opening Balance Equity", Type: "EQUITY", Parent: "3000", Postable: true, SystemRole: SystemRoleOpeningBalanceEquity},
		{Code: "4000", Name: "Revenue", Type: "INCOME"},
		{Code: "4900", Name: "Other Income", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "5000", Name: "Cost of Sales / Direct Costs", Type: "EXPENSE"},
		{Code: "6000", Name: "Operating Expenses", Type: "EXPENSE"},
		{Code: "6100", Name: "Payroll & Staff", Type: "EXPENSE", Parent: "6000"},
		{Code: "6110", Name: "Salaries & Wages", Type: "EXPENSE", Parent: "6100", Postable: true},
		{Code: "6120", Name: "Staff Benefits", Type: "EXPENSE", Parent: "6100", Postable: true},
		{Code: "6200", Name: "Occupancy", Type: "EXPENSE", Parent: "6000"},
		{Code: "6210", Name: "Rent", Type: "EXPENSE", Parent: "6200", Postable: true},
		{Code: "6220", Name: "Utilities", Type: "EXPENSE", Parent: "6200", Postable: true},
		{Code: "6300", Name: "Marketing & Selling", Type: "EXPENSE", Parent: "6000"},
		{Code: "6310", Name: "Advertising & Promotion", Type: "EXPENSE", Parent: "6300", Postable: true},
		{Code: "6400", Name: "Delivery & Transport", Type: "EXPENSE", Parent: "6000"},
		{Code: "6410", Name: "Delivery & Transport Expense", Type: "EXPENSE", Parent: "6400", Postable: true},
		{Code: "6500", Name: "Administrative & Finance", Type: "EXPENSE", Parent: "6000"},
		{Code: "6510", Name: "Bank & Payment Fees", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6520", Name: "Office Supplies", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6530", Name: "Software & Subscriptions", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6540", Name: "Phone & Internet", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6550", Name: "Professional Fees", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6560", Name: "Repairs & Maintenance", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6570", Name: "Depreciation Expense", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6580", Name: "Insurance", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6590", Name: "Taxes, Licenses & Fees", Type: "EXPENSE", Parent: "6500", Postable: true},
		{Code: "6900", Name: "Miscellaneous Expense", Type: "EXPENSE", Parent: "6000", Postable: true},
	}
	return append(base, extra...)
}

func myanKafeAccounts() []chartAccount {
	return []chartAccount{
		{Code: "4110", Name: "Coffee & Beverage Sales", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4120", Name: "Food & Product Sales", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4130", Name: "Merchandise Sales", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4210", Name: "Service & Other Operating Revenue", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "5110", Name: "Coffee & Product Cost", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5120", Name: "Food & Beverage Cost", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5130", Name: "Packaging Materials", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5140", Name: "Direct Fulfilment Cost", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5190", Name: "Other Direct Costs", Type: "EXPENSE", Parent: "5000", Postable: true},
	}
}

func royalMasterpieceAccounts() []chartAccount {
	return []chartAccount{
		{Code: "4110", Name: "Flower & Arrangement Sales", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4120", Name: "Surprise Box Sales", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4130", Name: "Gift, Toy, Balloon & Cake Sales", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4210", Name: "Decoration Service Revenue", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4220", Name: "Class & Workshop Revenue", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "5110", Name: "Flowers & Floral Materials", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5120", Name: "Gifts, Toys, Balloons & Cakes Cost", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5130", Name: "Packaging & Wrapping", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5140", Name: "Direct Delivery & Setup", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5190", Name: "Other Direct Costs", Type: "EXPENSE", Parent: "5000", Postable: true},
	}
}

func personalChart() []chartAccount {
	return []chartAccount{
		{Code: "1000", Name: "Assets", Type: "ASSET"},
		{Code: "1100", Name: "Cash & Cash Equivalents", Type: "ASSET", Parent: "1000"},
		{Code: "1110", Name: "Cash on Hand", Type: "ASSET", Parent: "1100", Postable: true},
		{Code: "1120", Name: "Bank Accounts", Type: "ASSET", Parent: "1100"},
		{Code: "1130", Name: "Mobile Wallets", Type: "ASSET", Parent: "1100"},
		{Code: "1200", Name: "Receivables", Type: "ASSET", Parent: "1000"},
		{Code: "1210", Name: "Other Receivables", Type: "ASSET", Parent: "1200", Postable: true},
		{Code: "1250", Name: "Due from Related Entities", Type: "ASSET", Parent: "1000"},
		{Code: "1400", Name: "Prepayments & Deposits", Type: "ASSET", Parent: "1000"},
		{Code: "1410", Name: "Prepaid Expenses", Type: "ASSET", Parent: "1400", Postable: true},
		{Code: "1420", Name: "Deposits", Type: "ASSET", Parent: "1400", Postable: true},
		{Code: "1500", Name: "Investments", Type: "ASSET", Parent: "1000"},
		{Code: "1510", Name: "Investments", Type: "ASSET", Parent: "1500", Postable: true},
		{Code: "1600", Name: "Personal Property", Type: "ASSET", Parent: "1000"},
		{Code: "1610", Name: "Vehicle", Type: "ASSET", Parent: "1600", Postable: true},
		{Code: "1620", Name: "Equipment & Electronics", Type: "ASSET", Parent: "1600", Postable: true},
		{Code: "2000", Name: "Liabilities", Type: "LIABILITY"},
		{Code: "2100", Name: "Current Liabilities", Type: "LIABILITY", Parent: "2000"},
		{Code: "2110", Name: "Credit Card Payable", Type: "LIABILITY", Parent: "2100", Postable: true},
		{Code: "2120", Name: "Personal Loans Payable", Type: "LIABILITY", Parent: "2100", Postable: true},
		{Code: "2250", Name: "Due to Related Entities", Type: "LIABILITY", Parent: "2000"},
		{Code: "2300", Name: "Long-term Liabilities", Type: "LIABILITY", Parent: "2000"},
		{Code: "2310", Name: "Long-term Loans", Type: "LIABILITY", Parent: "2300", Postable: true},
		{Code: "3000", Name: "Net Worth / Equity", Type: "EQUITY"},
		{Code: "3100", Name: "Personal Capital / Net Worth", Type: "EQUITY", Parent: "3000", Postable: true},
		{Code: "3980", Name: "Opening Balance Adjustment", Type: "EQUITY", Parent: "3000", Postable: true, SystemRole: SystemRoleOpeningBalanceAdjustment},
		{Code: "3990", Name: "Opening Balance Equity", Type: "EQUITY", Parent: "3000", Postable: true, SystemRole: SystemRoleOpeningBalanceEquity},
		{Code: "4000", Name: "Income", Type: "INCOME"},
		{Code: "4110", Name: "Salary & Wages", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4120", Name: "Business & Side Income", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4130", Name: "Interest & Investment Income", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4140", Name: "Rental & Other Earned Income", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "4900", Name: "Other Income", Type: "INCOME", Parent: "4000", Postable: true},
		{Code: "5000", Name: "Living Expenses", Type: "EXPENSE"},
		{Code: "5110", Name: "Housing & Rent", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5120", Name: "Food & Groceries", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5130", Name: "Utilities & Communications", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5140", Name: "Transportation", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5150", Name: "Health & Medical", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5160", Name: "Education", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5170", Name: "Travel & Leisure", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5180", Name: "Family, Gifts & Donations", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "5190", Name: "Personal Shopping & Miscellaneous", Type: "EXPENSE", Parent: "5000", Postable: true},
		{Code: "6000", Name: "Financial & Other Expenses", Type: "EXPENSE"},
		{Code: "6110", Name: "Bank & Payment Fees", Type: "EXPENSE", Parent: "6000", Postable: true},
		{Code: "6120", Name: "Insurance", Type: "EXPENSE", Parent: "6000", Postable: true},
		{Code: "6130", Name: "Taxes & Government Fees", Type: "EXPENSE", Parent: "6000", Postable: true},
		{Code: "6140", Name: "Interest Expense", Type: "EXPENSE", Parent: "6000", Postable: true},
		{Code: "6900", Name: "Other Expense", Type: "EXPENSE", Parent: "6000", Postable: true},
	}
}

func SeedChartOfAccounts(ctx context.Context, tx pgx.Tx, entityID, userID, template string) error {
	accounts, err := chartTemplate(template)
	if err != nil {
		return err
	}
	idsByCode := map[string]string{}
	for _, account := range accounts {
		id, err := ids.UUIDv7()
		if err != nil {
			return err
		}
		pub, err := ids.ULID()
		if err != nil {
			return err
		}
		var parent any
		if account.Parent != "" {
			parentID, ok := idsByCode[account.Parent]
			if !ok {
				return fmt.Errorf("chart template parent %s is missing for %s", account.Parent, account.Code)
			}
			parent = parentID
		}
		var systemRole any
		if account.SystemRole != "" {
			systemRole = account.SystemRole
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO accounts(
  id,public_id,entity_id,code,name,parent_id,account_type,system_role,is_postable,is_system,created_by
) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			id, pub, entityID, account.Code, account.Name, parent, account.Type, systemRole, account.Postable, account.SystemRole != "", userID); err != nil {
			return err
		}
		idsByCode[account.Code] = id
	}
	return nil
}
