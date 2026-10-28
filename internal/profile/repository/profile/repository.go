package profile

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/postgres"
	"github.com/jackc/pgx/v5"
)

type PgxCliPool interface {
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

type ProfileRepository struct {
	pool PgxCliPool
}

func NewProfileRepository(pool PgxCliPool) *ProfileRepository {
	return &ProfileRepository{pool: pool}
}

func (r *ProfileRepository) UpdateProfile(ctx context.Context, p *models.ProfileUpdate) error {
	const updateUser = `
		UPDATE "user"
		SET username = $2, nickname = $3, email = $4
		WHERE id = $1
	`
	const updateAuthor = `
		UPDATE author
		SET bio = $2, category = $3, payout_provider = $4, payout_account_id = $5
		WHERE id = $1
	`

	return pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, updateUser, p.UserID, p.Username, p.Nickname, p.Email)
		if err != nil {
			return parseUpdateError(err)
		}
		if result.RowsAffected() == 0 {
			return models.ErrProfileNotFound
		}

		if p.Author == nil {
			return nil
		}

		a := p.Author
		result, err = tx.Exec(ctx, updateAuthor, p.UserID, a.Bio, a.Category, a.PayoutProvider, a.PayoutAccountID)
		if err != nil {
			return parseUpdateError(err)
		}
		if result.RowsAffected() == 0 {
			return models.ErrNotAuthor
		}
		return nil
	})
}

func parseUpdateError(err error) error {
	err = postgres.ParseError(err)
	switch {
	case errors.Is(err, postgres.ErrUniqViolation):
		return models.ErrProfileConflict
	case errors.Is(err, postgres.ErrCheckViolation):
		return models.ErrProfileValidation
	}
	return fmt.Errorf("update profile error: %w", err)
}
