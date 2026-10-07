package memory

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/google/uuid"
)

type ProfileRepository struct {
	authorsMu sync.RWMutex
	authors   map[uuid.UUID]models.Author

	postsMu sync.RWMutex
	posts   map[uuid.UUID]models.Post
}

func NewProfileRepository() *ProfileRepository {
	ivanID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	alexandraID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	firstPostID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	secondPostID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")

	baseTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	firstPublishedAt := baseTime.Add(time.Hour)
	secondPublishedAt := baseTime.Add(25 * time.Hour)

	return &ProfileRepository{
		authors: map[uuid.UUID]models.Author{
			ivanID:      {Bio: "Я автор Иван с двумя постами", Category: "Технологии"},
			alexandraID: {Bio: "Я автор Александра без постов", Category: "Здоровье"},
		},
		posts: map[uuid.UUID]models.Post{
			firstPostID: {
				ID: firstPostID,
				AuthorID: ivanID,
				Title: "Первый пост",
				Body:   "Привет, это мой первый пост и в нем я расскажу о чем нибудь.",
				Status: models.PostStatusPublished,
				PublishedAt: &firstPublishedAt,
				CreatedAt: baseTime,
			},
			secondPostID: {
				ID: secondPostID,
				AuthorID: ivanID,
				Title: "Второй пост",
				Body: "Это второй пост",
				Status: models.PostStatusPublished,
				PublishedAt: &secondPublishedAt,
				CreatedAt: baseTime.Add(24 * time.Hour),
			},
		},
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
