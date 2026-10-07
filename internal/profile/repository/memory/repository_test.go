package memory

import (
	"context"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	repoAuthorID      = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	repoOtherAuthorID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	repoUnknownID     = uuid.MustParse("00000000-0000-4000-8000-000000000099")
	repoPostID1       = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	repoPostID2       = uuid.MustParse("00000000-0000-4000-8000-000000000102")
	repoPostID3       = uuid.MustParse("00000000-0000-4000-8000-000000000103")
	repoPostID4       = uuid.MustParse("00000000-0000-4000-8000-000000000104")
	repoTime          = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

func TestNewProfileRepository(t *testing.T) {
	t.Parallel()

	repo := NewProfileRepository()
	ivanID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	author, err := repo.GetAuthorByUserID(context.Background(), ivanID)
	require.NoError(t, err)
	require.Equal(t, "Технологии", author.Category)
	posts, err := repo.ListPostsByAuthorID(context.Background(), ivanID)
	require.NoError(t, err)
	require.Len(t, posts, 2)
}

func TestProfileRepository_GetAuthorByUserID_Success(t *testing.T) {
	t.Parallel()

	author := models.Author{Bio: "Пишу о книгах", Category: "Книги"}
	repo := &ProfileRepository{authors: map[uuid.UUID]models.Author{repoAuthorID: author}}
	got, err := repo.GetAuthorByUserID(context.Background(), repoAuthorID)
	require.NoError(t, err)
	require.Equal(t, &author, got)
}

func TestProfileRepository_GetAuthorByUserID_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		userID  uuid.UUID
		wantErr error
	}{
		{name: "unknown author", userID: repoUnknownID, wantErr: profileusecase.ErrAuthorNotFound},
		{name: "nil user ID", userID: uuid.Nil, wantErr: profileusecase.ErrAuthorNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &ProfileRepository{}
			got, err := repo.GetAuthorByUserID(context.Background(), tt.userID)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestProfileRepository_ListPostsByAuthorID_Success(t *testing.T) {
	t.Parallel()

	older := models.Post{ID: repoPostID1, AuthorID: repoAuthorID, Title: "Старый", CreatedAt: repoTime}
	newerFirst := models.Post{ID: repoPostID2, AuthorID: repoAuthorID, Title: "Новый А", CreatedAt: repoTime.Add(time.Hour)}
	newerSecond := models.Post{ID: repoPostID3, AuthorID: repoAuthorID, Title: "Новый Б", CreatedAt: repoTime.Add(time.Hour)}
	other := models.Post{ID: repoPostID4, AuthorID: repoOtherAuthorID, Title: "Чужой", CreatedAt: repoTime.Add(2 * time.Hour)}
	tests := []struct {
		name     string
		authorID uuid.UUID
		posts    map[uuid.UUID]models.Post
		want     []models.Post
	}{
		{name: "own posts sorted by date and ID", authorID: repoAuthorID, posts: map[uuid.UUID]models.Post{older.ID: older, newerSecond.ID: newerSecond, other.ID: other, newerFirst.ID: newerFirst}, want: []models.Post{newerFirst, newerSecond, older}},
		{name: "author without posts", authorID: repoOtherAuthorID, posts: map[uuid.UUID]models.Post{older.ID: older, newerSecond.ID: newerSecond, newerFirst.ID: newerFirst}, want: []models.Post{}},
		{name: "unknown author has no posts", authorID: repoUnknownID, posts: map[uuid.UUID]models.Post{older.ID: older, newerSecond.ID: newerSecond, other.ID: other, newerFirst.ID: newerFirst}, want: []models.Post{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &ProfileRepository{authors: map[uuid.UUID]models.Author{
				repoAuthorID: {}, repoOtherAuthorID: {},
			}, posts: tt.posts}
			got, err := repo.ListPostsByAuthorID(context.Background(), tt.authorID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
