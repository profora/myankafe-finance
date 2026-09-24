package httpapi

import "testing"

func TestRoleMatrix(t *testing.T) {
	roles := []string{"OWNER", "ADMIN", "ACCOUNTANT", "BOOKKEEPER", "VIEWER"}

	for _, role := range roles {
		t.Run(role, func(t *testing.T) {
			wantConfigure := role == "OWNER" || role == "ADMIN" || role == "ACCOUNTANT"
			if got := canConfigureAccounting(role); got != wantConfigure {
				t.Fatalf("canConfigureAccounting(%s)=%v want %v", role, got, wantConfigure)
			}

			wantOperate := role != "VIEWER"
			if got := canOperateLedger(role); got != wantOperate {
				t.Fatalf("canOperateLedger(%s)=%v want %v", role, got, wantOperate)
			}

			wantCorrect := role == "OWNER" || role == "ACCOUNTANT"
			if got := canCorrectPostedAccounting(role); got != wantCorrect {
				t.Fatalf("canCorrectPostedAccounting(%s)=%v want %v", role, got, wantCorrect)
			}
		})
	}
}
