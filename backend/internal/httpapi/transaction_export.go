package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/profora/myankafe-finance/backend/internal/repository/postgres"
)

func (s *Server) exportTransactionsCSV(w http.ResponseWriter, r *http.Request) {
	a := getAccess(r)
	q := r.URL.Query()

	status := q.Get("status")
	switch status {
	case "", "DRAFT", "POSTED", "VOIDED":
	default:
		fail(w, 400, errString("invalid status filter"))
		return
	}
	typ := q.Get("type")
	switch typ {
	case "", "INCOME", "EXPENSE", "ACCOUNT_TRANSFER", "INTER_ENTITY", "MANUAL_JOURNAL", "ADJUSTMENT", "REVERSAL":
	default:
		fail(w, 400, errString("invalid type filter"))
		return
	}

	filter := postgres.TransactionListFilter{
		Search:             q.Get("q"),
		Status:             status,
		Type:               typ,
		From:               q.Get("from"),
		To:                 q.Get("to"),
		FinancialAccountID: q.Get("financial_account_id"),
		Limit:              500,
	}

	filename := safeCSVFilename(a.Entity.Code + "-transactions-" + time.Now().UTC().Format("20060102") + ".csv")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Cache-Control", "no-store")

	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{
		"Date", "Movement", "Type", "Status", "Description", "Contact",
		"Financial Account", "Account Currency", "Signed Movement", "Balance",
		"Attachment Count", "Transaction ULID",
	})

	offset := 0
	for {
		filter.Offset = offset
		result, err := s.Store.ListTransactionsFiltered(r.Context(), a.Entity.ID, filter)
		if err != nil {
			return
		}
		for _, item := range result.Items {
			contact := ""
			if item.ContactName != nil {
				contact = *item.ContactName
			}
			financial := ""
			if item.FinancialAccount != nil {
				financial = *item.FinancialAccount
			}
			accountCurrency := ""
			if item.AccountCurrency != nil {
				accountCurrency = *item.AccountCurrency
			}
			movement := ""
			if item.SignedMovement != nil {
				movement = *item.SignedMovement
			}
			balance := ""
			if item.Balance != nil {
				balance = *item.Balance
			}
			_ = writer.Write([]string{
				item.Date, item.MovementLabel, item.Type, item.Status, item.Description, contact,
				financial, accountCurrency, movement, balance,
				strconv.Itoa(item.AttachmentCount), item.PublicID,
			})
		}
		offset += len(result.Items)
		if len(result.Items) == 0 || !result.HasMore {
			break
		}
		if offset >= 100000 {
			break
		}
	}
}

func safeCSVFilename(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "transactions.csv"
	}
	var b strings.Builder
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if !strings.HasSuffix(strings.ToLower(out), ".csv") {
		out += ".csv"
	}
	return out
}
