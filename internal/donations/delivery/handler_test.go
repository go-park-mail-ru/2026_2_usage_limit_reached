package donationhandlers_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/delivery/mock_donations_delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/cursor"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	defaultLimit = 20
	requestKey   = "00000000-0000-4000-8000-000000000501"
)

var (
	donationPayerID  = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	donationAuthorID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	donationOrderID  = uuid.MustParse("00000000-0000-4000-8000-000000000301")
	donationTime     = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	donationMessage  = "Спасибо за посты!"
	anonymous        = (*uuid.UUID)(nil)
	noCursor         = (*dto.DonationCursor)(nil)
	technicalErr     = errors.New("donation storage unavailable")

	donateTarget = "/authors/" + donationAuthorID.String() + "/donations"
	donateJSON   = `{"request_key":"` + requestKey + `","message":"Спасибо за посты!","is_anonymous":false,"amount_minor":50000,"currency":"RUB"}`
	donateReq    = dto.CreateDonationRequest{
		AuthorID:    donationAuthorID,
		RequestKey:  requestKey,
		Message:     &donationMessage,
		AmountMinor: 50000,
		Currency:    "RUB",
	}
)

func TestDonationHandler_SendDonation_Success(t *testing.T) {
	t.Parallel()

	router, uc := newRouter(t)
	uc.EXPECT().Donate(mock.Anything, donationPayerID, donateReq).
		Return(&dto.SentDonation{PaymentOrderID: donationOrderID, Status: "paid"}, nil)

	rec := serve(router, newRequest(http.MethodPost, donateTarget, donateJSON, &donationPayerID))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t,
		dto.CreateDonationResponse{PaymentOrderID: donationOrderID, Status: "paid"},
		testutils.DecodeJSON[dto.CreateDonationResponse](t, rec),
	)
}

func TestDonationHandler_SendDonation_AuthorFromPathOnly(t *testing.T) {
	t.Parallel()

	body := `{"author_id":"` + uuid.NewString() + `","request_key":"` + requestKey + `","amount_minor":50000,"currency":"RUB"}`
	want := dto.CreateDonationRequest{AuthorID: donationAuthorID, RequestKey: requestKey, AmountMinor: 50000, Currency: "RUB"}
	router, uc := newRouter(t)
	uc.EXPECT().Donate(mock.Anything, donationPayerID, want).
		Return(&dto.SentDonation{PaymentOrderID: donationOrderID, Status: "pending"}, nil)

	rec := serve(router, newRequest(http.MethodPost, donateTarget, body, &donationPayerID))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDonationHandler_SendDonation_InvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		body   string
	}{
		{name: "invalid author id", target: "/authors/not-a-uuid/donations", body: donateJSON},
		{name: "malformed json", target: donateTarget, body: `{"request_key":`},
		{name: "empty body", target: donateTarget, body: ``},
		{name: "wrong type", target: donateTarget, body: `{"request_key":"` + requestKey + `","amount_minor":"много","currency":"RUB"}`},
		{name: "amount below minimum", target: donateTarget, body: `{"request_key":"` + requestKey + `","amount_minor":4,"currency":"RUB"}`},
		{name: "unsupported currency", target: donateTarget, body: `{"request_key":"` + requestKey + `","amount_minor":50000,"currency":"USD"}`},
		{name: "missing currency", target: donateTarget, body: `{"request_key":"` + requestKey + `","amount_minor":50000}`},
		{name: "request key is not uuid", target: donateTarget, body: `{"request_key":"donation-1","amount_minor":50000,"currency":"RUB"}`},
		{name: "missing request key", target: donateTarget, body: `{"amount_minor":50000,"currency":"RUB"}`},
		{name: "too long message", target: donateTarget, body: `{"request_key":"` + requestKey + `","message":"` + strings.Repeat("я", 5000) + `","amount_minor":50000,"currency":"RUB"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, _ := newRouter(t)

			rec := serve(router, newRequest(http.MethodPost, tt.target, tt.body, &donationPayerID))

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestDonationHandler_SendDonation_Anonymous(t *testing.T) {
	t.Parallel()

	router, _ := newRouter(t)

	rec := serve(router, newRequest(http.MethodPost, donateTarget, donateJSON, anonymous))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDonationHandler_SendDonation_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ucErr      error
		wantStatus int
	}{
		{name: "self donation", ucErr: donationusecase.ErrForbidden, wantStatus: http.StatusForbidden},
		{name: "unknown author", ucErr: donationusecase.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "invalid donation", ucErr: donationusecase.ErrValidation, wantStatus: http.StatusBadRequest},
		{name: "request key reused", ucErr: donationusecase.ErrConflict, wantStatus: http.StatusConflict},
		{name: "payment failure", ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			uc.EXPECT().Donate(mock.Anything, donationPayerID, donateReq).Return(nil, wrapped(tt.ucErr))

			rec := serve(router, newRequest(http.MethodPost, donateTarget, donateJSON, &donationPayerID))

			require.Equal(t, tt.wantStatus, rec.Code)
			require.NotEmpty(t, testutils.DecodeJSON[response.ErrorResponse](t, rec).Error)
		})
	}
}

type listEndpoint struct {
	name   string
	target string
	expect func(uc *mock_donations_delivery.MockDonationUsecase, c *dto.DonationCursor, limit int) *mock.Call
}

func listEndpoints() []listEndpoint {
	return []listEndpoint{
		{
			name: "sent", target: "/profile/me/donations/sent",
			expect: func(uc *mock_donations_delivery.MockDonationUsecase, c *dto.DonationCursor, limit int) *mock.Call {
				return uc.EXPECT().GetSentDonations(mock.Anything, donationPayerID, c, limit).Call
			},
		},
		{
			name: "received", target: "/profile/me/donations/received",
			expect: func(uc *mock_donations_delivery.MockDonationUsecase, c *dto.DonationCursor, limit int) *mock.Call {
				return uc.EXPECT().GetReceivedDonations(mock.Anything, donationPayerID, c, limit).Call
			},
		},
	}
}

func TestDonationHandler_ListDonations_Pages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		withNext bool
	}{
		{name: "page with next cursor", withNext: true},
		{name: "last page"},
	}
	for _, e := range listEndpoints() {
		for _, tt := range tests {
			t.Run(e.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				items := donations(2)
				page := &dto.DonationPage{Items: items}
				if tt.withNext {
					page.Next = &dto.DonationCursor{CreatedOrPaidAt: items[1].CreatedOrPaidAt, DonationID: items[1].PaymentOrderID}
				}
				router, uc := newRouter(t)
				e.expect(uc, noCursor, defaultLimit).Return(page, nil)

				rec := serve(router, newRequest(http.MethodGet, e.target, "", &donationPayerID))

				require.Equal(t, http.StatusOK, rec.Code)
				got := testutils.DecodeJSON[dto.DonationPageResponse](t, rec)
				require.Equal(t, items, got.Items)
				if !tt.withNext {
					require.Nil(t, got.NextCursor)

					return
				}
				require.NotNil(t, got.NextCursor)
				require.Equal(t, encodeCursor(t, page.Next), *got.NextCursor)
			})
		}
	}
}

func TestDonationHandler_ListDonations_CursorAndLimit(t *testing.T) {
	t.Parallel()

	cur := &dto.DonationCursor{CreatedOrPaidAt: donationTime, DonationID: donationOrderID}
	for _, e := range listEndpoints() {
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			e.expect(uc, cur, 5).Return(&dto.DonationPage{Items: donations(1)}, nil)

			rec := serve(router, newRequest(http.MethodGet, e.target+"?limit=5&cursor="+encodeCursor(t, cur), "", &donationPayerID))

			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestDonationHandler_ListDonations_LimitOutOfRange(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		for _, limit := range []string{"0", "-1", "51", "много"} {
			t.Run(e.name+"/"+limit, func(t *testing.T) {
				t.Parallel()

				router, uc := newRouter(t)
				e.expect(uc, noCursor, defaultLimit).Return(&dto.DonationPage{}, nil)

				rec := serve(router, newRequest(http.MethodGet, e.target+"?limit="+limit, "", &donationPayerID))

				require.Equal(t, http.StatusOK, rec.Code)
			})
		}
	}
}

func TestDonationHandler_ListDonations_InvalidCursor(t *testing.T) {
	t.Parallel()

	foreignCodec := cursor.Codec[dto.DonationCursor]{Kind: "timed_posts", Valid: func(*dto.DonationCursor) bool { return true }}
	foreign, err := foreignCodec.Encode(&dto.DonationCursor{CreatedOrPaidAt: donationTime, DonationID: donationOrderID})
	require.NoError(t, err)

	cursors := map[string]string{
		"not base64":   "not*base64!",
		"not json":     "bm90LWpzb24",
		"other cursor": *foreign,
	}
	for _, e := range listEndpoints() {
		for name, c := range cursors {
			t.Run(e.name+"/"+name, func(t *testing.T) {
				t.Parallel()

				router, _ := newRouter(t)

				rec := serve(router, newRequest(http.MethodGet, e.target+"?cursor="+c, "", &donationPayerID))

				require.Equal(t, http.StatusBadRequest, rec.Code)
			})
		}
	}
}

func TestDonationHandler_ListDonations_InvalidNextCursor(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			e.expect(uc, noCursor, defaultLimit).Return(&dto.DonationPage{Items: donations(1), Next: &dto.DonationCursor{}}, nil)

			rec := serve(router, newRequest(http.MethodGet, e.target, "", &donationPayerID))

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestDonationHandler_ListDonations_Anonymous(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()

			router, _ := newRouter(t)

			rec := serve(router, newRequest(http.MethodGet, e.target, "", anonymous))

			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestDonationHandler_ListDonations_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ucErr      error
		wantStatus int
	}{
		{name: "unknown user", ucErr: donationusecase.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "storage failure", ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, e := range listEndpoints() {
		for _, tt := range tests {
			t.Run(e.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				router, uc := newRouter(t)
				e.expect(uc, noCursor, defaultLimit).Return(nil, wrapped(tt.ucErr))

				rec := serve(router, newRequest(http.MethodGet, e.target, "", &donationPayerID))

				require.Equal(t, tt.wantStatus, rec.Code)
			})
		}
	}
}
