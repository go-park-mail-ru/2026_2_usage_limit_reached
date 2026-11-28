package repository

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/monetization/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *MonetizationRepository) GetReceivedDonations(ctx context.Context, authorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*models.Donation, error) {
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

func (r *MonetizationRepository) GetSentDonations(ctx context.Context, donorID uuid.UUID, cursor *dto.DonationCursor, limit int) ([]*models.Donation, error) {
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

func (r *MonetizationRepository) MarkPaid(ctx context.Context, paymentOrderID uuid.UUID) error {
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
