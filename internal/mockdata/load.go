package mockdata

import (
	"fmt"
	"time"

	authmodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/models"
	authmemory "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/repository/memory"
	profilemodels "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profilememory "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/memory"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const Password = "DemoPassword123!"

func Load() (*authmemory.UserRepository, *profilememory.ProfileRepository, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hash mock password: %w", err)
	}

	readerID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	authorID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	emptyAuthorID := uuid.MustParse("00000000-0000-4000-8000-000000000003")
	createdAt := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)

	users, err := authmemory.NewUserRepositoryWithMockData([]authmodels.User{
		{ID: readerID, Username: "demo_reader", Nickname: "Читатель", Email: "reader@example.test", PasswordHash: string(hash), Status: "active", CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: authorID, Username: "demo_author", Nickname: "Александра", Email: "author@example.test", PasswordHash: string(hash), Status: "active", CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: emptyAuthorID, Username: "demo_empty_author", Nickname: "Новый автор", Email: "empty-author@example.test", PasswordHash: string(hash), Status: "active", CreatedAt: createdAt, UpdatedAt: createdAt},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load mock users: %w", err)
	}

	publishedAt := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	profiles, err := profilememory.NewProfileRepositoryWithMockData(
		map[uuid.UUID]profilemodels.Author{
			authorID:      {Bio: "Пишу о книгах и творчестве.", Category: "Искусство"},
			emptyAuthorID: {Bio: "Скоро здесь появятся публикации.", Category: "Образование"},
		},
		[]profilemodels.Post{
			{
				ID:          uuid.MustParse("00000000-0000-4000-8000-000000000101"),
				AuthorID:    authorID,
				Title:       "Новый выпуск",
				Body:        "Спасибо за поддержку! В этом выпуске — планы на месяц.",
				Status:      profilemodels.PostStatusPublished,
				PublishedAt: &publishedAt,
				CreatedAt:   publishedAt,
			},
			{
				ID:        uuid.MustParse("00000000-0000-4000-8000-000000000102"),
				AuthorID:  authorID,
				Title:     "Черновик следующего выпуска",
				Body:      "Текст ещё редактируется.",
				Status:    profilemodels.PostStatusDraft,
				CreatedAt: time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC),
			},
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("load mock profiles: %w", err)
	}
	return users, profiles, nil
}
