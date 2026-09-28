package seed

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Data struct {
	User    models.User
	Authors map[uuid.UUID]domain.Author
	Posts   map[uuid.UUID]domain.Post
}

func GetMockData() (Data, error) {
	userID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	publishedPostID := uuid.MustParse("00000000-0000-4000-8000-000000000101")
	draftPostID := uuid.MustParse("00000000-0000-4000-8000-000000000102")
	publishedAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("demo-password"), bcrypt.DefaultCost)
	if err != nil {
		return Data{}, err
	}
	return Data{
		User: models.User{
			ID: userID, Username: "mock", Nickname: "Mock", Email: "mock@example.com",
			PasswordHash: string(passwordHash), Status: "active", CreatedAt: publishedAt.Add(-31 * 24 * time.Hour),
		},
		Authors: map[uuid.UUID]domain.Author{
			userID: {ID: userID, Bio: "Mock-автор", Category: "Mock-категория", CreatedAt: publishedAt.Add(-24 * time.Hour)},
		},
		Posts: map[uuid.UUID]domain.Post{
			publishedPostID: {
				ID: publishedPostID, AuthorID: userID, Title: "Первый пост", Body: "Пример содержимого поста",
				Status: domain.PostStatusPublished, PublishedAt: &publishedAt,
				CreatedAt: publishedAt.Add(-time.Hour), UpdatedAt: publishedAt,
			},
			draftPostID: {
				ID: draftPostID, AuthorID: userID, Title: "Черновик", Body: "Пример черновика",
				Status: domain.PostStatusDraft, CreatedAt: publishedAt.Add(time.Hour), UpdatedAt: publishedAt.Add(time.Hour),
			},
		},
	}, nil
}
