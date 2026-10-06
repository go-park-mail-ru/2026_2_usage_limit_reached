package dto_test

import (
	"encoding/json"
	"testing"
	"time"

	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	mapperUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	mapperPostID = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	mapperTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

func TestToProfileResponse_Success(t *testing.T) {
	t.Parallel()

	user := &authdto.UserInfo{ID: mapperUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: mapperTime}
	tests := []struct {
		name     string
		author   *models.Author
		want     *profiledto.ProfileResponse
		wantJSON string
	}{
		{
			name:   "author is present",
			author: &models.Author{Bio: "Текст автора", Category: "Технологии"},
			want: &profiledto.ProfileResponse{
				User:   profiledto.ProfileUserResponse{ID: mapperUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: mapperTime},
				Author: &profiledto.AuthorResponse{Bio: "Текст автора", Category: "Технологии"},
			},
			wantJSON: `{"user":{"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_key":"avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"},"author":{"bio":"Текст автора","category":"Технологии"}}`,
		},
		{
			name:     "author is nil",
			want:     &profiledto.ProfileResponse{User: profiledto.ProfileUserResponse{ID: mapperUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: mapperTime}},
			wantJSON: `{"user":{"id":"00000000-0000-4000-8000-000000000001","username":"ivan001","nickname":"Иван","email":"ivan@example.test","avatar_key":"avatars/ivan.png","created_at":"2026-09-01T12:00:00Z"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := profiledto.ToProfileResponse(user, tt.author)
			require.Equal(t, tt.want, got)
			if tt.author == nil {
				require.Nil(t, got.Author)
			}
			data, err := json.Marshal(got)
			require.NoError(t, err)
			require.JSONEq(t, tt.wantJSON, string(data))
		})
	}
}

func TestToProfilePostsResponse_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		posts    []models.Post
		want     *profiledto.ProfilePostsResponse
		wantJSON string
	}{
		{
			name:     "published post has date",
			posts:    []models.Post{{ID: mapperPostID, AuthorID: mapperUserID, Title: "Пост", Body: "Текст", Status: models.PostStatusPublished, PublishedAt: &mapperTime}},
			want:     &profiledto.ProfilePostsResponse{UserID: mapperUserID, Posts: []profiledto.PostResponse{{ID: mapperPostID, Title: "Пост", Body: "Текст", Status: "published", PublishedAt: &mapperTime}}},
			wantJSON: `{"user_id":"00000000-0000-4000-8000-000000000001","posts":[{"id":"00000000-0000-4000-8000-000000000101","title":"Пост","body":"Текст","status":"published","published_at":"2026-09-01T12:00:00Z"}]}`,
		},
		{
			name:     "draft has nil date",
			posts:    []models.Post{{ID: mapperPostID, AuthorID: mapperUserID, Title: "Черновик", Body: "Текст", Status: models.PostStatusDraft}},
			want:     &profiledto.ProfilePostsResponse{UserID: mapperUserID, Posts: []profiledto.PostResponse{{ID: mapperPostID, Title: "Черновик", Body: "Текст", Status: "draft"}}},
			wantJSON: `{"user_id":"00000000-0000-4000-8000-000000000001","posts":[{"id":"00000000-0000-4000-8000-000000000101","title":"Черновик","body":"Текст","status":"draft"}]}`,
		},
		{
			name:     "no posts returns array",
			want:     &profiledto.ProfilePostsResponse{UserID: mapperUserID, Posts: []profiledto.PostResponse{}},
			wantJSON: `{"user_id":"00000000-0000-4000-8000-000000000001","posts":[]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := profiledto.ToProfilePostsResponse(mapperUserID, tt.posts)
			require.NotNil(t, got.Posts)
			require.Equal(t, tt.want, got)
			if len(tt.want.Posts) == 0 {
				require.Empty(t, got.Posts)
			}
			data, err := json.Marshal(got)
			require.NoError(t, err)
			require.JSONEq(t, tt.wantJSON, string(data))
		})
	}
}
