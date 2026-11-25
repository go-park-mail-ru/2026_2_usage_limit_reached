package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	paymentdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	paymentmodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	donationusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase/mock_monetization_usecase"
	mock_payments_usecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/usecase/mock_monetization_usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/payment_provider/mock_pkg_payment_provider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const pageLimit = 2

var (
	donationPayerID  = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	donationAuthorID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	donationOrderID  = uuid.MustParse("00000000-0000-4000-8000-000000000301")

	donationTime = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

	donationMessage = "Спасибо за посты!"

	technicalErr = errors.New("donation storage unavailable")

	donationCursor = &dto.DonationCursor{
		CreatedAt:  donationTime,
		DonationID: donationOrderID,
	}

	donationReq = dto.CreateDonationRequest{
		AuthorID:    donationAuthorID,
		RequestKey:  "00000000-0000-4000-8000-000000000501",
		Message:     &donationMessage,
		AmountMinor: 50000,
		Currency:    "RUB",
	}
)

func paymentOrder(status paymentmodels.PaymentOrderStatus) *paymentmodels.PaymentOrder {
	return &paymentmodels.PaymentOrder{
		ID:                  donationOrderID,
		PayerID:             donationPayerID,
		Purpose:             paymentmodels.PurposeDonation,
		Origin:              paymentmodels.OriginManual,
		ExpectedAmountMinor: 50000,
		Currency:            "RUB",
		Status:              status,
	}
}

func donationRecord() *models.Donation {
	return &models.Donation{
		PaymentOrderID: donationOrderID,
		DonorID:        donationPayerID,
		AuthorID:       donationAuthorID,
		Message:        &donationMessage,
		IsAnonymous:    false,
		CreatedAt:      donationTime,
		UpdatedAt:      donationTime,
	}
}

func authorInfo() *identitycontracts.UserInfo {
	return &identitycontracts.UserInfo{
		ID:        donationAuthorID,
		Username:  "author",
		Nickname:  "Автор",
		AvatarURL: "avatars/author.png",
	}
}

func createPaymentRequestMatcher() interface{} {
	return mock.MatchedBy(func(req paymentdto.CreateDonationWithPayment) bool {
		if req.ExpiresAt == nil {
			return false
		}

		now := time.Now()

		return req.DonorID == donationPayerID &&
			req.AuthorID == donationReq.AuthorID &&
			req.RequestKey == donationReq.RequestKey &&
			req.AmountMinor == donationReq.AmountMinor &&
			req.Currency == donationReq.Currency &&
			req.ExpiresAt.After(now.Add(14*time.Minute)) &&
			req.ExpiresAt.Before(now.Add(16*time.Minute))
	})
}

func createDonationRequestMatcher() interface{} {
	return mock.MatchedBy(func(req *dto.CreateDonationRequest) bool {
		return req != nil &&
			req.AuthorID == donationReq.AuthorID &&
			req.IsAnonymous == donationReq.IsAnonymous &&
			equalOptionalString(req.Message, donationReq.Message)
	})
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}

func expectDonationPrefix(
	repo *mock_monetization_usecase.MockDonationRepository,
	identity *mock_monetization_usecase.MockIdentityClient,
) []*mock.Call {
	identityCall := identity.EXPECT().
		GetUsers(mock.Anything, []uuid.UUID{donationAuthorID}).
		Return(map[uuid.UUID]*identitycontracts.UserInfo{
			donationAuthorID: authorInfo(),
		}, nil).Call

	createDonationCall := repo.On("CreateDonationWithPayment", mock.Anything, createPaymentRequestMatcher()).
		Return(donationRecord(), paymentOrder(paymentmodels.StatusPending), nil)

	return []*mock.Call{
		identityCall,
		createDonationCall,
	}
}

func expectPaymentResult(
	repo *mock_payments_usecase.MockPaymentRepo,
	provider *mock_pkg_payment_provider.MockProvider,
	status string,
) []*mock.Call {
	switch status {
	case "paid":
		repoStart := repo.EXPECT().StartAttempt(mock.Anything, donationOrderID).
			Return(paymentOrder(paymentmodels.StatusProcessing), &paymentmodels.PaymentAttempt{
				ID: attemptID, PaymentOrderID: donationOrderID, Status: paymentmodels.AttemptStatusProcessing,
			}, true, nil).Call
		charge := provider.EXPECT().Charge(mock.Anything, paymentprovider.ChargeRequest{
			AttemptID: attemptID, AmountMinor: donationReq.AmountMinor, Currency: donationReq.Currency,
		}).Return(paymentprovider.ChargeResult{
			Status: paymentprovider.ChargeSucceeded, ExternalID: "tx-1",
			AmountMinor: donationReq.AmountMinor, Currency: donationReq.Currency,
		}, nil).Call
		complete := repo.EXPECT().CompleteSuccessfulAttempt(mock.Anything, mock.Anything).Return(false, nil).Call
		final := repo.EXPECT().GetOrderByID(mock.Anything, donationOrderID).
			Return(paymentOrder(paymentmodels.StatusPaid), nil).Call
		return []*mock.Call{repoStart, charge, complete, final}
	case "failed":
		repoStart := repo.EXPECT().StartAttempt(mock.Anything, donationOrderID).
			Return(paymentOrder(paymentmodels.StatusProcessing), &paymentmodels.PaymentAttempt{
				ID: attemptID, PaymentOrderID: donationOrderID, Status: paymentmodels.AttemptStatusProcessing,
			}, true, nil).Call
		charge := provider.EXPECT().Charge(mock.Anything, paymentprovider.ChargeRequest{
			AttemptID: attemptID, AmountMinor: donationReq.AmountMinor, Currency: donationReq.Currency,
		}).Return(paymentprovider.ChargeResult{Status: paymentprovider.ChargeFailed}, nil).Call
		fail := repo.EXPECT().FailAttempt(mock.Anything, donationOrderID, attemptID, "").Return(nil).Call
		final := repo.EXPECT().GetOrderByID(mock.Anything, donationOrderID).
			Return(paymentOrder(paymentmodels.StatusFailed), nil).Call
		return []*mock.Call{repoStart, charge, fail, final}
	default:
		repoStart := repo.EXPECT().StartAttempt(mock.Anything, donationOrderID).
			Return(paymentOrder(paymentmodels.StatusProcessing), &paymentmodels.PaymentAttempt{
				ID: attemptID, PaymentOrderID: donationOrderID, Status: paymentmodels.AttemptStatusProcessing,
			}, false, nil).Call
		return []*mock.Call{repoStart}
	}
}

func expectedSentDonation(status string) *dto.SentDonation {
	avatar := "avatars/author.png"

	return &dto.SentDonation{
		PaymentOrderID: donationOrderID,
		Message:        &donationMessage,
		IsAnonymous:    false,
		AmountMinor:    50000,
		Currency:       "RUB",
		Status:         status,
		CreatedAt:      donationTime,
		Author: &dto.DonationUserInfo{
			Username:  "author",
			Nickname:  "Автор",
			AvatarUrl: &avatar,
		},
	}
}

func TestMonetizationUsecase_Donate_PaymentStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		payStatus string
		markPaid  bool
	}{
		{
			name:      "payment succeeded",
			payStatus: "paid",
			markPaid:  true,
		},
		{
			name:      "payment failed",
			payStatus: "failed",
		},
		{
			name:      "payment still processing",
			payStatus: "processing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, paymentRepo, provider, identity := newUsecase(t)

			calls := expectDonationPrefix(repo, identity)
			calls = append(calls, expectPaymentResult(paymentRepo, provider, tt.payStatus)...)

			if tt.markPaid {
				calls = append(calls,
					repo.EXPECT().
						MarkPaid(mock.Anything, donationOrderID).
						Return(nil).Call,
				)
			}

			mock.InOrder(calls...)

			got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

			require.NoError(t, err)
			require.Equal(t, expectedSentDonation(tt.payStatus), got)
		})
	}
}

func TestMonetizationUsecase_Donate_InvalidIDs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		donorID uuid.UUID
		req     dto.CreateDonationRequest
		wantErr error
	}{
		{
			name:    "nil donor ID",
			donorID: uuid.Nil,
			req:     donationReq,
			wantErr: donationusecase.ErrValidation,
		},
		{
			name:    "nil author ID",
			donorID: donationPayerID,
			req: dto.CreateDonationRequest{
				RequestKey: donationReq.RequestKey,
			},
			wantErr: donationusecase.ErrValidation,
		},
		{
			name:    "self donation",
			donorID: donationPayerID,
			req: func() dto.CreateDonationRequest {
				req := donationReq
				req.AuthorID = donationPayerID
				return req
			}(),
			wantErr: donationusecase.ErrForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, _, _, _, _ := newUsecase(t)

			got, err := uc.Donate(context.Background(), tt.donorID, tt.req)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestMonetizationUsecase_Donate_InvalidRequest(t *testing.T) {
	t.Parallel()

	uc, _, _, _, _ := newUsecase(t)
	req := donationReq
	req.AmountMinor = 0

	got, err := uc.Donate(context.Background(), donationPayerID, req)

	require.Nil(t, got)
	require.ErrorIs(t, err, donationusecase.ErrValidation)
}

func TestMonetizationUsecase_Donate_AuthorNotFound(t *testing.T) {
	t.Parallel()

	uc, _, _, _, identity := newUsecase(t)

	identity.EXPECT().
		GetUsers(mock.Anything, []uuid.UUID{donationAuthorID}).
		Return(map[uuid.UUID]*identitycontracts.UserInfo{}, nil)

	got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

	require.Nil(t, got)
	require.ErrorIs(t, err, donationusecase.ErrNotFound)
}

func TestMonetizationUsecase_Donate_IdentityError(t *testing.T) {
	t.Parallel()

	uc, _, _, _, identity := newUsecase(t)

	identity.EXPECT().
		GetUsers(mock.Anything, []uuid.UUID{donationAuthorID}).
		Return(nil, technicalErr)

	got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

	require.Nil(t, got)
	require.ErrorIs(t, err, technicalErr)
}

func TestMonetizationUsecase_Donate_CreatePaymentError(t *testing.T) {
	t.Parallel()

	uc, repo, _, _, identity := newUsecase(t)

	identity.EXPECT().
		GetUsers(mock.Anything, []uuid.UUID{donationAuthorID}).
		Return(map[uuid.UUID]*identitycontracts.UserInfo{
			donationAuthorID: authorInfo(),
		}, nil)

	repo.On("CreateDonationWithPayment", mock.Anything, createPaymentRequestMatcher()).
		Return(nil, nil, technicalErr)

	got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

	require.Nil(t, got)
	require.ErrorIs(t, err, technicalErr)
}

func TestMonetizationUsecase_Donate_InvalidPaymentOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{name: "empty transaction result"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, _, _, identity := newUsecase(t)

			identity.EXPECT().
				GetUsers(mock.Anything, []uuid.UUID{donationAuthorID}).
				Return(map[uuid.UUID]*identitycontracts.UserInfo{
					donationAuthorID: authorInfo(),
				}, nil)

			repo.On("CreateDonationWithPayment", mock.Anything, createPaymentRequestMatcher()).
				Return(nil, nil, nil)

			got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

			require.Nil(t, got)
			require.Error(t, err)
		})
	}
}

func TestMonetizationUsecase_Donate_RepositoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{
			name:    "unknown donation",
			repoErr: models.ErrDonationNotFound,
			wantErr: donationusecase.ErrNotFound,
		},
		{
			name:    "unknown author",
			repoErr: models.ErrAuthorNotFound,
			wantErr: donationusecase.ErrNotFound,
		},
		{
			name:    "request key reused",
			repoErr: models.ErrIdempotencyKeyReused,
			wantErr: donationusecase.ErrConflict,
		},
		{
			name:    "invalid donation",
			repoErr: models.ErrDonationValidation,
			wantErr: donationusecase.ErrValidation,
		},
		{
			name:    "storage failure",
			repoErr: technicalErr,
			wantErr: technicalErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, repo, _, _, identity := newUsecase(t)

			identityCall := identity.EXPECT().
				GetUsers(mock.Anything, []uuid.UUID{donationAuthorID}).
				Return(map[uuid.UUID]*identitycontracts.UserInfo{
					donationAuthorID: authorInfo(),
				}, nil).Call

			createDonationCall := repo.On("CreateDonationWithPayment", mock.Anything, createPaymentRequestMatcher()).
				Return(nil, nil, tt.repoErr)

			mock.InOrder(identityCall, createDonationCall)

			got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestMonetizationUsecase_Donate_PaymentError(t *testing.T) {
	t.Parallel()

	uc, repo, paymentRepo, provider, identity := newUsecase(t)

	calls := expectDonationPrefix(repo, identity)
	calls = append(calls, paymentRepo.EXPECT().StartAttempt(mock.Anything, donationOrderID).
		Return(paymentOrder(paymentmodels.StatusProcessing), &paymentmodels.PaymentAttempt{
			ID: attemptID, PaymentOrderID: donationOrderID,
			Status: paymentmodels.AttemptStatusProcessing,
		}, true, nil).Call)
	provider.EXPECT().Charge(mock.Anything, mock.Anything).Return(paymentprovider.ChargeResult{}, technicalErr)

	mock.InOrder(calls...)

	got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

	require.Nil(t, got)
	require.ErrorIs(t, err, technicalErr)
}

func TestMonetizationUsecase_Donate_MarkPaidError(t *testing.T) {
	t.Parallel()

	uc, repo, paymentRepo, provider, identity := newUsecase(t)

	calls := expectDonationPrefix(repo, identity)
	calls = append(calls, expectPaymentResult(paymentRepo, provider, "paid")...)
	calls = append(calls,
		repo.EXPECT().
			MarkPaid(mock.Anything, donationOrderID).
			Return(technicalErr).Call,
	)

	mock.InOrder(calls...)

	got, err := uc.Donate(context.Background(), donationPayerID, donationReq)

	require.Nil(t, got)
	require.ErrorIs(t, err, technicalErr)
}

type listMethod struct {
	name     string
	userID   uuid.UUID
	received bool
	expect   func(
		repo *mock_monetization_usecase.MockDonationRepository,
		cursor *dto.DonationCursor,
		limit int,
		items []*models.Donation,
		err error,
	)
	call func(
		uc *donationusecase.MonetizationUsecase,
		cursor *dto.DonationCursor,
		limit int,
	) (*dto.DonationPage, error)
}

func listMethods() []listMethod {
	ctx := context.Background()

	return []listMethod{
		{
			name:     "GetSentDonations",
			userID:   donationPayerID,
			received: false,
			expect: func(
				repo *mock_monetization_usecase.MockDonationRepository,
				cursor *dto.DonationCursor,
				limit int,
				items []*models.Donation,
				err error,
			) {
				repo.EXPECT().
					GetSentDonations(mock.Anything, donationPayerID, cursor, limit+1).
					Return(items, err)
			},
			call: func(
				uc *donationusecase.MonetizationUsecase,
				cursor *dto.DonationCursor,
				limit int,
			) (*dto.DonationPage, error) {
				return uc.GetSentDonations(ctx, donationPayerID, cursor, limit)
			},
		},
		{
			name:     "GetReceivedDonations",
			userID:   donationAuthorID,
			received: true,
			expect: func(
				repo *mock_monetization_usecase.MockDonationRepository,
				cursor *dto.DonationCursor,
				limit int,
				items []*models.Donation,
				err error,
			) {
				repo.EXPECT().
					GetReceivedDonations(mock.Anything, donationAuthorID, cursor, limit+1).
					Return(items, err)
			},
			call: func(
				uc *donationusecase.MonetizationUsecase,
				cursor *dto.DonationCursor,
				limit int,
			) (*dto.DonationPage, error) {
				return uc.GetReceivedDonations(ctx, donationAuthorID, cursor, limit)
			},
		},
	}
}

func listPaymentStatus(i int) string {
	statuses := []string{"paid", "failed", "processing"}
	return statuses[i%len(statuses)]
}

func listUserID(donation *models.Donation, received bool) uuid.UUID {
	if received {
		return donation.DonorID
	}

	return donation.AuthorID
}

func listUserInfo(id uuid.UUID) *identitycontracts.UserInfo {
	return &identitycontracts.UserInfo{
		ID:        id,
		Username:  "donation-user",
		Nickname:  "Пользователь",
		AvatarURL: "avatar.png",
	}
}

func listDonationOrders(
	items []*models.Donation,
) ([]uuid.UUID, map[uuid.UUID]*paymentmodels.PaymentOrder) {
	orderIDs := make([]uuid.UUID, 0, len(items))
	orders := make(map[uuid.UUID]*paymentmodels.PaymentOrder, len(items))

	for i, donation := range items {
		orderIDs = append(orderIDs, donation.PaymentOrderID)
		orders[donation.PaymentOrderID] = &paymentmodels.PaymentOrder{
			ID:                  donation.PaymentOrderID,
			ExpectedAmountMinor: int64(50000 + i),
			Currency:            "RUB",
			Status:              paymentmodels.PaymentOrderStatus(listPaymentStatus(i)),
		}
	}

	return orderIDs, orders
}

func listIdentityUsers(
	items []*models.Donation,
	received bool,
) ([]uuid.UUID, map[uuid.UUID]*identitycontracts.UserInfo) {
	userIDs := make([]uuid.UUID, 0, len(items))
	users := make(map[uuid.UUID]*identitycontracts.UserInfo, len(items))
	seen := make(map[uuid.UUID]struct{}, len(items))

	for _, donation := range items {
		if received && donation.IsAnonymous {
			continue
		}

		id := listUserID(donation, received)
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		userIDs = append(userIDs, id)
		users[id] = listUserInfo(id)
	}

	return userIDs, users
}

func expectListEnrichment(
	payments *mock_payments_usecase.MockPaymentRepo,
	identity *mock_monetization_usecase.MockIdentityClient,
	items []*models.Donation,
	received bool,
) {
	orderIDs, orders := listDonationOrders(items)
	userIDs, users := listIdentityUsers(items, received)

	paymentCall := payments.EXPECT().
		GetOrdersByIDs(mock.Anything, orderIDs).
		Return(orders, nil).Call

	identityCall := identity.EXPECT().
		GetUsers(mock.Anything, userIDs).
		Return(users, nil).Call

	mock.InOrder(paymentCall, identityCall)
}

func expectedDonationResponses(
	items []*models.Donation,
	received bool,
) []*dto.GetDonationResponse {
	_, users := listIdentityUsers(items, received)

	result := make([]*dto.GetDonationResponse, 0, len(items))

	for i, donation := range items {
		var user *dto.DonationUserInfo

		if !received || !donation.IsAnonymous {
			if info, ok := users[listUserID(donation, received)]; ok {
				avatar := info.AvatarURL
				var avatarURL *string
				if avatar != "" {
					avatarURL = &avatar
				}

				user = &dto.DonationUserInfo{
					Username:  info.Username,
					Nickname:  info.Nickname,
					AvatarUrl: avatarURL,
				}
			}
		}

		result = append(result, &dto.GetDonationResponse{
			PaymentOrderID: donation.PaymentOrderID,
			Message:        donation.Message,
			IsAnonymous:    donation.IsAnonymous,
			AmountMinor:    int64(50000 + i),
			Currency:       "RUB",
			Status:         listPaymentStatus(i),
			CreatedAt:      donation.CreatedAt,
			User:           user,
		})
	}

	return result
}

func TestMonetizationUsecase_ListDonations_Pagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fetched   int
		wantItems int
		wantNext  bool
	}{
		{
			name:      "empty page",
			fetched:   0,
			wantItems: 0,
		},
		{
			name:      "last full page",
			fetched:   pageLimit,
			wantItems: pageLimit,
		},
		{
			name:      "extra item becomes next cursor",
			fetched:   pageLimit + 1,
			wantItems: pageLimit,
			wantNext:  true,
		},
	}

	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				items := donations(tt.fetched)
				uc, repo, payments, _, identity := newUsecase(t)

				m.expect(repo, donationCursor, pageLimit, items, nil)

				pageItems := items[:tt.wantItems]
				if len(pageItems) > 0 {
					expectListEnrichment(payments, identity, pageItems, m.received)
				}

				got, err := m.call(uc, donationCursor, pageLimit)

				require.NoError(t, err)
				require.Equal(
					t,
					expectedDonationResponses(pageItems, m.received),
					got.Items,
				)

				if !tt.wantNext {
					require.Nil(t, got.Next)
					return
				}

				last := pageItems[len(pageItems)-1]
				require.Equal(t, &dto.DonationCursor{
					CreatedAt:  last.CreatedAt,
					DonationID: last.PaymentOrderID,
				}, got.Next)
			})
		}
	}
}

func TestMonetizationUsecase_ListDonations_RepositoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{
			name:    "unknown author",
			repoErr: models.ErrAuthorNotFound,
			wantErr: donationusecase.ErrNotFound,
		},
		{
			name:    "storage failure",
			repoErr: technicalErr,
			wantErr: technicalErr,
		},
	}

	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				uc, repo, _, _, _ := newUsecase(t)

				m.expect(repo, donationCursor, pageLimit, nil, tt.repoErr)

				got, err := m.call(uc, donationCursor, pageLimit)

				require.Nil(t, got)
				require.ErrorIs(t, err, tt.wantErr)
			})
		}
	}
}

func TestMonetizationUsecase_ListDonations_PaymentError(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			items := donations(1)
			uc, repo, payments, _, _ := newUsecase(t)

			m.expect(repo, nil, pageLimit, items, nil)

			payments.EXPECT().
				GetOrdersByIDs(mock.Anything, []uuid.UUID{items[0].PaymentOrderID}).
				Return(nil, technicalErr)

			got, err := m.call(uc, nil, pageLimit)

			require.Nil(t, got)
			require.ErrorIs(t, err, technicalErr)
		})
	}
}

func TestMonetizationUsecase_ListDonations_IdentityError(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			items := donations(1)
			uc, repo, payments, _, identity := newUsecase(t)

			m.expect(repo, nil, pageLimit, items, nil)

			_, orders := listDonationOrders(items)
			userIDs, _ := listIdentityUsers(items, m.received)

			payments.EXPECT().
				GetOrdersByIDs(mock.Anything, []uuid.UUID{items[0].PaymentOrderID}).
				Return(orders, nil)

			identity.EXPECT().
				GetUsers(mock.Anything, userIDs).
				Return(nil, technicalErr)

			got, err := m.call(uc, nil, pageLimit)

			require.Nil(t, got)
			require.ErrorIs(t, err, technicalErr)
		})
	}
}

func TestMonetizationUsecase_ListDonations_MissingPayment(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			items := donations(1)
			uc, repo, payments, _, identity := newUsecase(t)

			m.expect(repo, nil, pageLimit, items, nil)

			_, users := listIdentityUsers(items, m.received)

			payments.EXPECT().
				GetOrdersByIDs(mock.Anything, []uuid.UUID{items[0].PaymentOrderID}).
				Return(map[uuid.UUID]*paymentmodels.PaymentOrder{}, nil)

			userIDs, _ := listIdentityUsers(items, m.received)
			identity.EXPECT().
				GetUsers(mock.Anything, userIDs).
				Return(users, nil)

			got, err := m.call(uc, nil, pageLimit)

			require.Nil(t, got)
			require.Error(t, err)
		})
	}
}

func TestMonetizationUsecase_GetReceivedDonations_AnonymousDonor(t *testing.T) {
	t.Parallel()

	items := donations(1)
	items[0].IsAnonymous = true

	uc, repo, payments, _, identity := newUsecase(t)

	repo.EXPECT().
		GetReceivedDonations(mock.Anything, donationAuthorID, (*dto.DonationCursor)(nil), pageLimit+1).
		Return(items, nil)

	orderIDs, orders := listDonationOrders(items)

	payments.EXPECT().
		GetOrdersByIDs(mock.Anything, orderIDs).
		Return(orders, nil)

	identity.EXPECT().
		GetUsers(mock.Anything, []uuid.UUID{}).
		Return(map[uuid.UUID]*identitycontracts.UserInfo{}, nil)

	got, err := uc.GetReceivedDonations(
		context.Background(),
		donationAuthorID,
		nil,
		pageLimit,
	)

	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.True(t, got.Items[0].IsAnonymous)
	require.Nil(t, got.Items[0].User)
}
