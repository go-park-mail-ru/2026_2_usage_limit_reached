package profileusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/dto"
	authuc "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/auth/usecase"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	profileUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	profilePostID = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	profileTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

type userInfoStub struct {
	user *authdto.UserInfo
	err  error
}

func (s userInfoStub) FindUserByID(context.Context, uuid.UUID) (*authdto.UserInfo, error) {
	return s.user, s.err
}

type profileRepositoryStub struct {
	author    *models.Author
	authorErr error
	posts     []models.Post
	postsErr  error
}

func (s profileRepositoryStub) GetAuthorByUserID(context.Context, uuid.UUID) (*models.Author, error) {
	return s.author, s.authorErr
}

func (s profileRepositoryStub) ListPostsByAuthorID(context.Context, uuid.UUID) ([]models.Post, error) {
	return s.posts, s.postsErr
}

func TestProfileUsecase_GetMyProfile_Success(t *testing.T) {
	t.Parallel()

	user := &authdto.UserInfo{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: profileTime}
	author := &models.Author{Bio: "Пишу о технологиях", Category: "Технологии"}
	tests := []struct {
		name      string
		author    *models.Author
		authorErr error
		want      *profiledto.ProfileResponse
	}{
		{
			name:   "author profile",
			author: author,
			want: &profiledto.ProfileResponse{
				User:   profiledto.ProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: profileTime},
				Author: &profiledto.AuthorResponse{Bio: "Пишу о технологиях", Category: "Технологии"},
			},
		},
		{
			name:      "reader has no author profile",
			authorErr: profileusecase.ErrAuthorNotFound,
			want: &profiledto.ProfileResponse{
				User: profiledto.ProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: profileTime},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := profileusecase.NewProfileUsecase(userInfoStub{user: user}, profileRepositoryStub{author: tt.author, authorErr: tt.authorErr})
			got, err := uc.GetMyProfile(context.Background(), profileUserID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			if tt.want.Author == nil {
				require.Nil(t, got.Author)
			}
		})
	}
}

func TestProfileUsecase_GetMyProfile_Errors(t *testing.T) {
	t.Parallel()

	technicalUserErr := errors.New("user storage unavailable")
	technicalAuthorErr := errors.New("author storage unavailable")
	tests := []struct {
		name      string
		user      *authdto.UserInfo
		userErr   error
		authorErr error
		wantErr   error
	}{
		{name: "unknown user", userErr: authuc.ErrUserNotFound, wantErr: profileusecase.ErrProfileNotFound},
		{name: "user storage failure", userErr: technicalUserErr, wantErr: technicalUserErr},
		{name: "author storage failure", user: &authdto.UserInfo{ID: profileUserID}, authorErr: technicalAuthorErr, wantErr: technicalAuthorErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := profileusecase.NewProfileUsecase(userInfoStub{user: tt.user, err: tt.userErr}, profileRepositoryStub{authorErr: tt.authorErr})
			got, err := uc.GetMyProfile(context.Background(), profileUserID)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != profileusecase.ErrProfileNotFound {
				require.Equal(t, false, errors.Is(err, profileusecase.ErrProfileNotFound))
			}
		})
	}
}

func TestProfileUsecase_GetMyPosts_Success(t *testing.T) {
	t.Parallel()

	posts := []models.Post{{ID: profilePostID, AuthorID: profileUserID, Title: "Первый пост", Body: "Текст", Status: models.PostStatusPublished, PublishedAt: &profileTime, CreatedAt: profileTime}}
	tests := []struct {
		name  string
		posts []models.Post
		want  *profiledto.ProfilePostsResponse
	}{
		{
			name:  "author has posts",
			posts: posts,
			want: &profiledto.ProfilePostsResponse{UserID: profileUserID, Posts: []profiledto.PostResponse{
				{ID: profilePostID, Title: "Первый пост", Body: "Текст", Status: "published", PublishedAt: &profileTime},
			}},
		},
		{name: "author has no posts", want: &profiledto.ProfilePostsResponse{UserID: profileUserID, Posts: []profiledto.PostResponse{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := profileusecase.NewProfileUsecase(userInfoStub{}, profileRepositoryStub{posts: tt.posts})
			got, err := uc.GetMyPosts(context.Background(), profileUserID)
			require.NoError(t, err)
			require.NotNil(t, got.Posts)
			require.Equal(t, tt.want, got)
			if len(tt.want.Posts) == 0 {
				require.Empty(t, got.Posts)
			}
		})
	}
}

func TestProfileUsecase_GetMyPosts_Errors(t *testing.T) {
	t.Parallel()

	storageErr := errors.New("posts storage unavailable")
	tests := []struct {
		name     string
		postsErr error
		wantErr  error
	}{
		{name: "posts storage failure", postsErr: storageErr, wantErr: storageErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc := profileusecase.NewProfileUsecase(userInfoStub{}, profileRepositoryStub{postsErr: tt.postsErr})
			got, err := uc.GetMyPosts(context.Background(), profileUserID)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
