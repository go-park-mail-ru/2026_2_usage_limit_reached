package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/google/uuid"
)

type ProfileRepository struct {
	authorsMu sync.RWMutex
	authors  map[uuid.UUID]models.Author

	postsMu sync.RWMutex
	posts   map[uuid.UUID]models.Post
}

func NewProfileRepository() *ProfileRepository {
	return &ProfileRepository{
		authors: make(map[uuid.UUID]models.Author),
		posts:   make(map[uuid.UUID]models.Post),
	}
}

func (r *ProfileRepository) GetAuthorByUserID(ctx context.Context, userID uuid.UUID) (*models.Author, error) {
	r.authorsMu.RLock()
	defer r.authorsMu.RUnlock()

	author, ok := r.authors[userID]
	if !ok {
		return nil, profileusecase.ErrAuthorNotFound
	}

	return &author, nil
}

func (r *ProfileRepository) ListPostsByAuthorID(ctx context.Context, authorID uuid.UUID) ([]models.Post, error) {
	r.postsMu.RLock()
	defer r.postsMu.RUnlock()

	posts := make([]models.Post, 0)
	for _, post := range r.posts {
		if post.AuthorID == authorID {
			posts = append(posts, post)
		}
	}

	slices.SortFunc(posts, func(a, b models.Post) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID.String(), b.ID.String())
	})

	return posts, nil
}
