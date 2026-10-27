package donationrepository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/dto"
	models "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/donations/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type PgxCliPool interface {
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type DonationRepository struct {
	pool PgxCliPool
}

func NewDonationRepository(pool PgxCliPool) *DonationRepository {
	return &DonationRepository{pool: pool}
}

func (r *DonationRepository) CreateDonation(ctx context.Context, payerID uuid.UUID, req *dto.CreateDonationRequest) (*dto.SentDonation, error) {
	var result *dto.SentDonation

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		const insertOrder = `
			INSERT INTO payment_order (payer_id, request_key, purpose, origin, expected_amount_minor, currency, status)
			VALUES ($1, $2, 'donation', 'manual', $3, $4, 'pending')
			ON CONFLICT (request_key) DO NOTHING
			RETURNING id, created_at
		`
		var (
			orderID        uuid.UUID
			orderCreatedAt time.Time
		)
		err := tx.QueryRow(ctx, insertOrder, payerID, req.RequestKey, req.AmountMinor, req.Currency).Scan(&orderID, &orderCreatedAt)

		if errors.Is(err, pgx.ErrNoRows) {
			return tx.QueryRow(ctx, `SELECT id FROM payment_order WHERE request_key = $1`, req.RequestKey).Scan(&orderID)
		} else if err != nil {
			err = postgres.ParseError(err)
			if errors.Is(err, postgres.ErrFKViolation) {
				return models.ErrDonationValidation
			}
			return fmt.Errorf("create payment order: %w", err)
		}

		const insertDonation = `
			INSERT INTO donation (payment_order_id, author_id, message, is_anonymous)
			VALUES ($1, $2, $3, $4)
			RETURNING created_at
		`
		var donationCreatedAt time.Time
		err = tx.QueryRow(ctx, insertDonation, orderID, req.AuthorID, req.Message, req.IsAnonymous).Scan(&donationCreatedAt)
		if err != nil {
			err = postgres.ParseError(err)
			if errors.Is(err, postgres.ErrFKViolation) {
				return models.ErrDonationValidation
			}
			return fmt.Errorf("create donation: %w", err)
		}

		const insertAttempt = `
			INSERT INTO payment_attempt (payment_order_id, provider, status)
			VALUES ($1, 'mock', 'created')
			RETURNING id
		`
		var attemptID uuid.UUID
		err = tx.QueryRow(ctx, insertAttempt, orderID).Scan(&attemptID)
		if err != nil {
			return fmt.Errorf("create payment attempt: %w", err)
		}

		result = &dto.SentDonation{
			PaymentOrderID: orderID,
			Message:        req.Message,
			IsAnonymous:    req.IsAnonymous,
			AmountMinor:    req.AmountMinor,
			Currency:       req.Currency,
			Status:         "pending",
			CreatedAt:      donationCreatedAt,
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

func (r *DonationRepository) GetReceivedDonations(ctx context.Context, authorID uuid.UUID, limit int) ([]*dto.ReceivedDonation, error) {
	const query = `
		SELECT 
			d.payment_order_id,
			d.message,
			d.is_anonymous,
			po.expected_amount_minor,
			po.currency,
			po.paid_at,
    		CASE WHEN d.is_anonymous THEN NULL ELSE u.username END AS donor_username,
    		CASE WHEN d.is_anonymous THEN NULL ELSE u.avatar_key END AS donor_avatar_key
		FROM donation d
		JOIN payment_order po ON po.id = d.payment_order_id
		JOIN "user" u ON u.id = po.payer_id
		WHERE d.author_id = $1
		  AND po.status = 'paid'
		ORDER BY po.paid_at DESC, d.payment_order_id DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, authorID, limit)
	if err != nil {
		return nil, fmt.Errorf("get received donations: %w", err)
	}
	defer rows.Close()

	return scanReceivedDonations(rows)
}

func (r *DonationRepository) GetSentDonations(ctx context.Context, payerID uuid.UUID, limit int) ([]*dto.SentDonation, error) {
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
			u.avatar_key
		FROM donation d
		JOIN payment_order po ON po.id = d.payment_order_id
		JOIN author a ON a.id = d.author_id
		JOIN "user" u ON u.id = a.id
		WHERE po.payer_id = $1
		ORDER BY d.created_at DESC, d.payment_order_id DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, payerID, limit)
	if err != nil {
		return nil, fmt.Errorf("get sent donations: %w", err)
	}
	defer rows.Close()

	return scanSentDonations(rows)
}

func scanReceivedDonations(rows pgx.Rows) ([]*dto.ReceivedDonation, error) {
	var donations []*dto.ReceivedDonation
	for rows.Next() {
		d := &dto.ReceivedDonation{}
		var donorUsername, donorAvatar *string

		err := rows.Scan(
			&d.PaymentOrderID, &d.Message, &d.IsAnonymous,
			&d.AmountMinor, &d.Currency, &d.PaidAt,
			&donorUsername, &donorAvatar,
		)
		if err != nil {
			return nil, fmt.Errorf("scan received donation: %w", err)
		}

		if donorUsername != nil {
			d.Donor = &dto.DonationUserInfo{
				Username:  *donorUsername,
				AvatarKey: donorAvatar,
			}
		}
		donations = append(donations, d)
	}
	return donations, rows.Err()
}

func scanSentDonations(rows pgx.Rows) ([]*dto.SentDonation, error) {
	var donations []*dto.SentDonation
	for rows.Next() {
		d := &dto.SentDonation{}
		var (
			authorUsername string
			authorAvatar   *string
		)

		err := rows.Scan(
			&d.PaymentOrderID, &d.Message, &d.IsAnonymous,
			&d.AmountMinor, &d.Currency, &d.Status, &d.CreatedAt,
			&authorUsername, &authorAvatar,
		)
		if err != nil {
			return nil, fmt.Errorf("scan sent donation: %w", err)
		}

		d.Author = &dto.DonationUserInfo{
			Username:  authorUsername,
			AvatarKey: authorAvatar,
		}
		donations = append(donations, d)
	}
	return donations, rows.Err()
}
