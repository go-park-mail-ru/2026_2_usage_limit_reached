package donationhandlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	donationhandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/delivery/mock_donations_delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func newRouter(t *testing.T) (*mux.Router, *mock_donations_delivery.MockDonationUsecase) {
	t.Helper()

	uc := mock_donations_delivery.NewMockDonationUsecase(t)
	h := donationhandlers.NewHandler(uc, testutils.SetupValidator(), testutils.SetupLogger())
	router := mux.NewRouter()
	h.RegisterRoutes(router)
	return router, uc
}

func newRequest(method, target, body string, userID *uuid.UUID) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if userID != nil {
		r = testutils.WithUser(r, *userID)
	}
	return r
}

func serve(router http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func wrapped(err error) error {
	return fmt.Errorf("usecase: %w", err)
}

func donations(n int) []*dto.GetDonationResponse {
	items := make([]*dto.GetDonationResponse, n)
	for i := range items {
		items[i] = &dto.GetDonationResponse{
			PaymentOrderID:  uuid.New(),
			Message:         &donationMessage,
			AmountMinor:     50000,
			Currency:        "RUB",
			Status:          "paid",
			CreatedOrPaidAt: donationTime.Add(-time.Duration(i) * time.Hour),
			User:            &dto.DonationUserInfo{Username: "ivan001", Nickname: "Иван"},
		}
	}
	return items
}

func encodeCursor(t *testing.T, c *dto.DonationCursor) string {
	t.Helper()

	raw, err := donationhandlers.DonationsCursor.Encode(c)
	require.NoError(t, err)
	return *raw
}
