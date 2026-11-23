package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	donationhandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/delivery/http"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/delivery/http/mock_delivery_http"
	httprouter "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/delivery/http/router"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func newRouter(t *testing.T) (*mux.Router, *mock_delivery_http.MockDonationUsecase) {
	t.Helper()

	uc := mock_delivery_http.NewMockDonationUsecase(t)
	h := donationhandlers.NewDonationHandler(uc, testutils.SetupValidator(), testutils.SetupLogger())
	router := mux.NewRouter()
	httprouter.RegisterRoutes(router, httprouter.Handlers{Donation: h})
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
