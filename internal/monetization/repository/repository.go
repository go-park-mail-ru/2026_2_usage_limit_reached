package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DonationRepository struct {
	pool   postgres.DB
	logger *slog.Logger
}

func NewDonationRepository(db *postgres.DBClient, logger *slog.Logger) *DonationRepository {
	return &DonationRepository{
		pool:   db.Pool,
		logger: logger,
	}
}

func (r *DonationRepository) CreateDonation(ctx context.Context, paymentOrderID, donorID uuid.UUID, req *dto.CreateDonationRequest) (*models.Donation, error) {
	if paymentOrderID == uuid.Nil || donorID == uuid.Nil || req == nil || req.AuthorID == uuid.Nil {
		return nil, models.ErrDonationValidation
	}

	r.logger.DebugContext(ctx, "executing transaction",
		slog.String("op", "create donation"),
		slog.String("payment_order_id", paymentOrderID.String()),
		slog.String("donor_id", donorID.String()),
		slog.String("author_id", req.AuthorID.String()),
	)

	const insertDonation = `
		INSERT INTO donation (
			payment_order_id,
			donor_id,
			author_id,
			message,
			is_anonymous
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (payment_order_id) DO NOTHING
		RETURNING created_at, updated_at
	`

	const getExistingDonation = `
		SELECT
			payment_order_id,
			donor_id,
			author_id,
			message,
			is_anonymous,
			created_at,
			updated_at
		FROM donation
		WHERE payment_order_id = $1
	`

	result := &models.Donation{
		PaymentOrderID: paymentOrderID,
		DonorID:        donorID,
		AuthorID:       req.AuthorID,
		Message:        req.Message,
		IsAnonymous:    req.IsAnonymous,
	}

	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		err := tx.QueryRow(
			ctx,
			insertDonation,
			paymentOrderID,
			donorID,
			req.AuthorID,
			req.Message,
			req.IsAnonymous,
		).Scan(&result.CreatedAt, &result.UpdatedAt)

		if err == nil {
			return nil
		}

		if !errors.Is(err, pgx.ErrNoRows) {
			parsedErr := postgres.ParseError(err)
			if errors.Is(parsedErr, postgres.ErrFKViolation) ||
				errors.Is(parsedErr, postgres.ErrCheckViolation) ||
				errors.Is(parsedErr, postgres.ErrUniqViolation) {
				return models.ErrDonationValidation
			}

			return fmt.Errorf("insert donation: %w", err)
		}

		err = tx.QueryRow(ctx, getExistingDonation, paymentOrderID).Scan(
			&result.PaymentOrderID,
			&result.DonorID,
			&result.AuthorID,
			&result.Message,
			&result.IsAnonymous,
			&result.CreatedAt,
			&result.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.ErrDonationNotFound
			}

			return fmt.Errorf("get existing donation: %w", err)
		}

		if result.DonorID != donorID ||
			result.AuthorID != req.AuthorID ||
			!equalOptionalString(result.Message, req.Message) ||
			result.IsAnonymous != req.IsAnonymous {
			return models.ErrIdempotencyKeyReused
		}

		return nil
	},
	)
	if err != nil {
		parsedErr := postgres.ParseError(err)
		if errors.Is(parsedErr, postgres.ErrFKViolation) ||
			errors.Is(parsedErr, postgres.ErrCheckViolation) ||
			errors.Is(parsedErr, postgres.ErrUniqViolation) {
			return nil, models.ErrDonationValidation
		}

		if errors.Is(err, models.ErrDonationValidation) ||
			errors.Is(err, models.ErrDonationNotFound) ||
			errors.Is(err, models.ErrIdempotencyKeyReused) {
			return nil, err
		}

		return nil, fmt.Errorf("create donation transaction failed: %w", err)
	}

	return result, nil
}

func (r *DonationRepository) GetReceivedDonations(ctx context.Context, authorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*models.Donation, error) {
	const query = `
		SELECT
			payment_order_id,
			donor_id,
			author_id,
			message,
			is_anonymous,
			created_at,
			updated_at
		FROM donation
		WHERE author_id = $1
			AND paid = TRUE
			AND (
					$2::timestamptz IS NULL
					OR (created_at, payment_order_id) < ($2::timestamptz, $3::uuid)
			)
		ORDER BY created_at DESC, payment_order_id DESC
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get received donations"),
		slog.String("author_id", authorID.String()),
		slog.Int("limit", limit),
		slog.Bool("has_cursor", cursor != nil),
	)

	ts, id := donationCursorArgs(cursor)

	rows, err := r.pool.Query(ctx, query, authorID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get received donations: %w", err)
	}

	return scanDonations(rows)
}

func (r *DonationRepository) GetSentDonations(ctx context.Context, donorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*models.Donation, error) {
	const query = `
		SELECT
			payment_order_id,
			donor_id,
			author_id,
			message,
			is_anonymous,
			created_at,
			updated_at
		FROM donation
		WHERE donor_id = $1
		  AND (
				$2::timestamptz IS NULL
				OR (created_at, payment_order_id) < ($2::timestamptz, $3::uuid)
		  )
		ORDER BY created_at DESC, payment_order_id DESC
		LIMIT $4
	`

	r.logger.DebugContext(ctx, "executing query",
		slog.String("op", "get sent donations"),
		slog.String("donor_id", donorID.String()),
		slog.Int("limit", limit),
		slog.Bool("has_cursor", cursor != nil),
	)

	ts, id := donationCursorArgs(cursor)

	rows, err := r.pool.Query(ctx, query, donorID, ts, id, limit)
	if err != nil {
		return nil, fmt.Errorf("get sent donations: %w", err)
	}

	return scanDonations(rows)
}

func (r *DonationRepository) MarkPaid(ctx context.Context, paymentOrderID uuid.UUID) error {
	const markDonationAsPaid = `
		UPDATE donation
		SET paid = TRUE
		WHERE payment_order_id = $1
		AND paid = FALSE
	`

	_, err := r.pool.Exec(ctx, markDonationAsPaid, paymentOrderID)
	if err != nil {
		return fmt.Errorf("mark donation as paid: %w", err)
	}

	return nil
}

func donationCursorArgs(cursor *dto.DonationCursor) (*time.Time, *uuid.UUID) {
	if cursor == nil {
		return nil, nil
	}

	return &cursor.CreatedAt, &cursor.DonationID
}

func scanDonations(rows pgx.Rows) ([]*models.Donation, error) {
	defer rows.Close()

	donations := make([]*models.Donation, 0)

	for rows.Next() {
		donation := &models.Donation{}

		if err := rows.Scan(
			&donation.PaymentOrderID,
			&donation.DonorID,
			&donation.AuthorID,
			&donation.Message,
			&donation.IsAnonymous,
			&donation.CreatedAt,
			&donation.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan donation: %w", err)
		}

		donations = append(donations, donation)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate donations: %w", err)
	}

	return donations, nil
}

func equalOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}
