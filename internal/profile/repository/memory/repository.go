package memory

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/domain"
	"github.com/google/uuid"
)

type ProfileRepository struct {
	authors map[uuid.UUID]domain.Author
	posts   map[uuid.UUID]domain.Post
}

func NewProfileRepository(authors map[uuid.UUID]domain.Author, posts map[uuid.UUID]domain.Post) *ProfileRepository {
	return &ProfileRepository{
		authors: maps.Clone(authors),
		posts:   maps.Clone(posts),
	}
}

func (r *ProfileRepository) GetAuthorByUserID(ctx context.Context, userID uuid.UUID) (domain.Author, bool, error) {
	if err := ctx.Err(); err != nil {
		return domain.Author{}, false, err
	}
	author, ok := r.authors[userID]
	return author, ok, nil
}

func (r *ProfileRepository) ListPostsByAuthorID(ctx context.Context, authorID uuid.UUID) ([]domain.Post, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	posts := make([]domain.Post, 0)
	for _, post := range r.posts {
		if post.AuthorID == authorID {
			posts = append(posts, post)
		}
	}
	slices.SortFunc(posts, func(a, b domain.Post) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID.String(), b.ID.String())
	})
	return posts, nil
}
