package donationrepository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DonationRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewDonationRepository(db *postgres.DBClient, logger *slog.Logger) *DonationRepository {
	return &DonationRepository{pool: db.Pool, logger: logger}
}

func (r *DonationRepository) CreateDonation(ctx context.Context, payerID uuid.UUID, req *dto.CreateDonationRequest) (*dto.SentDonation, error) {
	const insertOrder = `
		INSERT INTO payment_order (payer_id, request_key, purpose, origin, expected_amount_minor, currency, status)
		VALUES ($1, $2, 'donation', 'manual', $3, $4, 'pending')
		ON CONFLICT (request_key) DO NOTHING
		RETURNING id
	`
	const insertDonation = `
		INSERT INTO donation (payment_order_id, author_id, message, is_anonymous)
		VALUES ($1, $2, $3, $4)
		RETURNING created_at
	`
	const insertAttempt = `
		INSERT INTO payment_attempt (payment_order_id, provider, status)
		VALUES ($1, 'mock', 'created')
		RETURNING id
	`

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "create donation"),
		slog.String("userID", payerID.String()),
		slog.String("authorID", req.AuthorID.String()),
		slog.String("request_key", req.RequestKey),
	)

	var result *dto.SentDonation
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		var orderID uuid.UUID
		err := tx.QueryRow(ctx, insertOrder, payerID, req.RequestKey, req.AmountMinor, req.Currency).Scan(&orderID)
		if errors.Is(err, pgx.ErrNoRows) {
			result, err = checkExistingOrder(ctx, tx, req)
			return err // do nothing on existing order
		} else if err != nil {
			err = postgres.ParseError(err)
			if errors.Is(err, postgres.ErrFKViolation) {
				return models.ErrDonationValidation
			}
			return fmt.Errorf("create payment order: %w", err)
		}

		var donationCreatedAt time.Time
		err = tx.QueryRow(ctx, insertDonation, orderID, req.AuthorID, req.Message, req.IsAnonymous).Scan(&donationCreatedAt)
		if err != nil {
			err = postgres.ParseError(err)
			if errors.Is(err, postgres.ErrFKViolation) {
				return models.ErrAuthorNotFound
			}
			return fmt.Errorf("create donation: %w", err)
		}

		var attemptID uuid.UUID
		err = tx.QueryRow(ctx, insertAttempt, orderID).Scan(&attemptID)
		if err != nil {
			return fmt.Errorf("create payment attempt: %w", err)
		}

		result = &dto.SentDonation{
			PaymentOrderID:   orderID,
			PaymentAttemptID: &attemptID,
			Message:          req.Message,
			IsAnonymous:      req.IsAnonymous,
			AmountMinor:      req.AmountMinor,
			Currency:         req.Currency,
			Status:           "pending",
			CreatedAt:        donationCreatedAt,
		}

		return nil
	})

	if err != nil {
		err = postgres.ParseError(err)
		if errors.Is(err, postgres.ErrCheckViolation) || errors.Is(err, postgres.ErrUniqViolation) {
			return nil, models.ErrDonationValidation
		}
		return nil, fmt.Errorf("create donation transaction failed: %w", err)
	}

	return result, nil
}

func checkExistingOrder(ctx context.Context, tx pgx.Tx, req *dto.CreateDonationRequest) (*dto.SentDonation, error) {
	const query = `
		SELECT po.id, d.author_id, d.message, d.is_anonymous, 
		po.expected_amount_minor, po.currency, po.status, d.created_at,
	       (SELECT pa.id FROM payment_attempt pa
	         WHERE pa.payment_order_id = po.id
	         ORDER BY pa.created_at DESC, pa.id DESC LIMIT 1) AS attempt_id
	  	FROM payment_order po
		JOIN donation d ON d.payment_order_id = po.id
		WHERE po.request_key = $1
	`

	var (
		authorID uuid.UUID
		donation dto.SentDonation
	)
	err := tx.QueryRow(ctx, query, req.RequestKey).Scan(
		&donation.PaymentOrderID,
		&authorID,
		&donation.Message,
		&donation.IsAnonymous,
		&donation.AmountMinor,
		&donation.Currency,
		&donation.Status,
		&donation.CreatedAt,
		&donation.PaymentAttemptID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrDonationNotFound
		}
		return nil, fmt.Errorf("error while get existing payment order: %w", err)
	}

	if authorID != req.AuthorID ||
		!equalPtr(donation.Message, req.Message) ||
		donation.AmountMinor != req.AmountMinor ||
		donation.Currency != req.Currency ||
		donation.IsAnonymous != req.IsAnonymous {
		return nil, models.ErrIdempotencyKeyReused
	}

	return &donation, nil
}

func equalPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (r *DonationRepository) GetReceivedDonations(ctx context.Context, authorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*dto.GetDonationResponse, error) {
	const query = `
		SELECT
			d.payment_order_id,
			d.message,
			d.is_anonymous,
			po.expected_amount_minor,
			po.currency,
			po.status,
			po.paid_at,
    		CASE WHEN d.is_anonymous THEN NULL ELSE u.username END AS donor_username,
			CASE WHEN d.is_anonymous THEN NULL ELSE u.nickname END AS donot_nickname,
    		CASE WHEN d.is_anonymous THEN NULL ELSE u.avatar_key END AS donor_avatar_key
		FROM donation d
		JOIN payment_order po ON po.id = d.payment_order_id
		JOIN "user" u ON u.id = po.payer_id
		WHERE d.author_id = $1
		  AND po.status = 'paid'
		  AND ($2::timestamptz IS NULL OR (po.paid_at, po.id) < ($2::timestamptz, $3::uuid))
		ORDER BY po.paid_at DESC, d.payment_order_id DESC
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get received donations"),
		slog.String("authorID", authorID.String()),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	var (
		ts *time.Time
		id *uuid.UUID
	)
	if cursor != nil {
		ts, id = &cursor.CreatedOrPaidAt, &cursor.DonationID
	}

	rows, err := r.pool.Query(ctx, query, authorID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get received donations: %w", err)
	}
	defer rows.Close()

	return scanReceivedDonations(rows)
}

func scanReceivedDonations(rows pgx.Rows) ([]*dto.GetDonationResponse, error) {
	donations := make([]*dto.GetDonationResponse, 0)
	for rows.Next() {
		d := &dto.GetDonationResponse{}
		var donorUsername, donorNickname, donorAvatar *string

		err := rows.Scan(
			&d.PaymentOrderID,
			&d.Message,
			&d.IsAnonymous,
			&d.AmountMinor,
			&d.Currency,
			&d.Status,
			&d.CreatedOrPaidAt,
			&donorUsername,
			&donorNickname,
			&donorAvatar,
		)
		if err != nil {
			return nil, fmt.Errorf("scan received donation: %w", err)
		}

		if donorUsername != nil {
			d.User = &dto.DonationUserInfo{
				Username:  *donorUsername,
				Nickname:  *donorNickname,
				AvatarUrl: donorAvatar,
			}
		}
		donations = append(donations, d)
	}
	return donations, rows.Err()
}

func (r *DonationRepository) GetSentDonations(ctx context.Context, payerID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*dto.GetDonationResponse, error) {
	const query = `
		SELECT
			d.payment_order_id,
			d.message,
			d.is_anonymous,
			po.expected_amount_minor,
			po.currency,
			po.status,
			d.created_at,
			u.username,
			u.nickname,
			u.avatar_key
		FROM donation d
		JOIN payment_order po ON po.id = d.payment_order_id
		JOIN "user" u ON u.id = d.author_id
		WHERE po.payer_id = $1
			AND ($2::timestamptz IS NULL OR (po.created_at, po.id) < ($2::timestamptz, $3::uuid))
		ORDER BY d.created_at DESC, d.payment_order_id DESC
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get sent donations"),
		slog.String("payerID", payerID.String()),
		slog.Int("limit", limit),
		slog.Bool("hasCursor", cursor != nil),
	)

	var (
		ts *time.Time
		id *uuid.UUID
	)
	if cursor != nil {
		ts, id = &cursor.CreatedOrPaidAt, &cursor.DonationID
	}

	rows, err := r.pool.Query(ctx, query, payerID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get sent donations: %w", err)
	}
	defer rows.Close()

	return scanSentDonations(rows)
}

func scanSentDonations(rows pgx.Rows) ([]*dto.GetDonationResponse, error) {
	donations := make([]*dto.GetDonationResponse, 0)
	for rows.Next() {
		d := &dto.GetDonationResponse{}
		var (
			authorUsername string
			authorNickname string
			authorAvatar   *string
		)

		err := rows.Scan(
			&d.PaymentOrderID,
			&d.Message,
			&d.IsAnonymous,
			&d.AmountMinor,
			&d.Currency,
			&d.Status,
			&d.CreatedOrPaidAt,
			&authorUsername,
			&authorNickname,
			&authorAvatar,
		)
		if err != nil {
			return nil, fmt.Errorf("scan sent donation: %w", err)
		}

		d.User = &dto.DonationUserInfo{
			Username:  authorUsername,
			Nickname:  authorNickname,
			AvatarUrl: authorAvatar,
		}
		donations = append(donations, d)
	}
	return donations, rows.Err()
}
