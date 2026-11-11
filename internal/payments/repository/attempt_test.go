package paymentrepository

import (
	"context"
	"testing"

	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/payments/models"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
	"github.com/stretchr/testify/require"
)

var attemptColumns = []string{
	"id", "payment_order_id", "provider", "external_transaction_id", "status",
	"charged_amount_minor", "charged_currency", "failure_reason", "completed_at", "created_at", "updated_at",
}

func TestPaymentRepository_GetAttemptByID(t *testing.T) {
	externalID, currency, reason := "ext-1", "RUB", "insufficient funds"
	amount := int64(15000)

	tests := []struct {
		name    string
		rows    *pgxmock.Rows
		err     error
		want    *models.PaymentAttempt
		wantErr error
	}{
		{
			name: "succeeded attempt with all fields",
			rows: pgxmock.NewRows(attemptColumns).AddRow(
				testAttemptID, testOrderID, "mock", &externalID, models.AttemptStatusSucceeded,
				&amount, &currency, &reason, &testTime, testTime, testTime,
			),
			want: &models.PaymentAttempt{
				ID:                    testAttemptID,
				PaymentOrderID:        testOrderID,
				Provider:              "mock",
				ExternalTransactionID: &externalID,
				Status:                models.AttemptStatusSucceeded,
				ChargedAmountMinor:    &amount,
				ChargedCurrency:       &currency,
				FailureReason:         &reason,
				CompletedAt:           &testTime,
				CreatedAt:             testTime,
				UpdatedAt:             testTime,
			},
		},
		{
			name: "created attempt without optional fields",
			rows: pgxmock.NewRows(attemptColumns).AddRow(
				testAttemptID, testOrderID, "mock", nil, models.AttemptStatusCreated,
				nil, nil, nil, nil, testTime, testTime,
			),
			want: &models.PaymentAttempt{
				ID:             testAttemptID,
				PaymentOrderID: testOrderID,
				Provider:       "mock",
				Status:         models.AttemptStatusCreated,
				CreatedAt:      testTime,
				UpdatedAt:      testTime,
			},
		},
		{name: "unknown attempt", err: pgx.ErrNoRows, wantErr: models.ErrPaymentAttemptNotFound},
		{name: "database error", err: errDB, wantErr: errDB},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, pool := newTestRepository(t)
			expect := pool.ExpectQuery("FROM payment_attempt").WithArgs(testAttemptID)
			if tt.err != nil {
				expect.WillReturnError(tt.err)
			} else {
				expect.WillReturnRows(tt.rows)
			}

			got, err := repo.GetAttemptByID(context.Background(), testAttemptID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			}
			require.NoError(t, pool.ExpectationsWereMet())
		})
	}
}
