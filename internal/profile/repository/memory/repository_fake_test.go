package memory_test

import (
	"context"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/repository/memory"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewProfileRepositoryWithMockData_Success(t *testing.T) {
	t.Parallel()

	author := models.Author{Bio: "Пишу о книгах", Category: "Книги"}
	post := models.Post{ID: repoPostID1, AuthorID: repoAuthorID, Title: "Первый пост", CreatedAt: repoTime}
	tests := []struct {
		name    string
		authors map[uuid.UUID]models.Author
		posts   []models.Post
		want    models.Post
	}{
		{name: "valid author and post", authors: map[uuid.UUID]models.Author{repoAuthorID: author}, posts: []models.Post{post}, want: post},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, err := memory.NewProfileRepositoryWithMockData(tt.authors, tt.posts)
			require.NoError(t, err)
			require.NotNil(t, repo)
			gotAuthor, err := repo.GetAuthorByUserID(context.Background(), repoAuthorID)
			require.NoError(t, err)
			require.Equal(t, &author, gotAuthor)
			gotPosts, err := repo.ListPostsByAuthorID(context.Background(), repoAuthorID)
			require.NoError(t, err)
			require.Equal(t, []models.Post{tt.want}, gotPosts)
		})
	}
}

func TestNewProfileRepositoryWithMockData_Errors(t *testing.T) {
	t.Parallel()

	post := models.Post{ID: repoPostID1, AuthorID: repoAuthorID, CreatedAt: repoTime}
	tests := []struct {
		name    string
		authors map[uuid.UUID]models.Author
		posts   []models.Post
		wantErr string
	}{
		{name: "nil author user ID", authors: map[uuid.UUID]models.Author{uuid.Nil: {}}, wantErr: "mock author has an empty user ID"},
		{name: "nil post ID", authors: map[uuid.UUID]models.Author{repoAuthorID: {}}, posts: []models.Post{{AuthorID: repoAuthorID}}, wantErr: "mock post has an empty ID"},
		{name: "post has unknown author", authors: map[uuid.UUID]models.Author{repoAuthorID: {}}, posts: []models.Post{{ID: repoPostID1, AuthorID: repoUnknownID}}, wantErr: "mock post 00000000-0000-4000-8000-000000000101 has no author"},
		{name: "duplicate post ID", authors: map[uuid.UUID]models.Author{repoAuthorID: {}}, posts: []models.Post{post, post}, wantErr: "duplicate mock post ID 00000000-0000-4000-8000-000000000101"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, err := memory.NewProfileRepositoryWithMockData(tt.authors, tt.posts)
			require.Nil(t, repo)
			require.Error(t, err)
			require.Equal(t, tt.wantErr, err.Error())
		})
	}
}
