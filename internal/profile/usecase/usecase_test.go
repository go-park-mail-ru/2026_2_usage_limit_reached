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

type userInfoMock struct {
	users map[uuid.UUID]authdto.UserInfo
	err   error
}

func (s userInfoMock) FindUserByID(_ context.Context, id uuid.UUID) (*authdto.UserInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	user, ok := s.users[id]
	if !ok {
		return nil, authuc.ErrUserNotFound
	}
	return &user, nil
}

type profileRepositoryMock struct {
	authors   map[uuid.UUID]models.Author
	authorErr error
	posts     map[uuid.UUID][]models.Post
	postsErr  error
}

func (s profileRepositoryMock) GetAuthorByUserID(_ context.Context, id uuid.UUID) (*models.Author, error) {
	if s.authorErr != nil {
		return nil, s.authorErr
	}
	author, ok := s.authors[id]
	if !ok {
		return nil, profileusecase.ErrAuthorNotFound
	}
	return &author, nil
}

func (s profileRepositoryMock) ListPostsByAuthorID(_ context.Context, id uuid.UUID) ([]models.Post, error) {
	if s.postsErr != nil {
		return nil, s.postsErr
	}
	return s.posts[id], nil
}

func TestProfileUsecase_GetMyProfile_Success(t *testing.T) {
	t.Parallel()

	user := &authdto.UserInfo{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: profileTime}
	author := &models.Author{Bio: "Пишу о технологиях", Category: "Технологии"}
	tests := []struct {
		name   string
		author *models.Author
		want   *profiledto.ProfileResponse
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
			name: "reader has no author profile",
			want: &profiledto.ProfileResponse{
				User: profiledto.ProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: "avatars/ivan.png", CreatedAt: profileTime},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authors := make(map[uuid.UUID]models.Author)
			if tt.author != nil {
				authors[profileUserID] = *tt.author
			}
			uc := profileusecase.NewProfileUsecase(
				userInfoMock{users: map[uuid.UUID]authdto.UserInfo{profileUserID: *user}},
				profileRepositoryMock{authors: authors},
			)
			got, err := uc.GetMyProfile(context.Background(), profileUserID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
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
			users := make(map[uuid.UUID]authdto.UserInfo)
			if tt.user != nil {
				users[profileUserID] = *tt.user
			}
			uc := profileusecase.NewProfileUsecase(userInfoMock{users: users, err: tt.userErr}, profileRepositoryMock{authorErr: tt.authorErr})
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
			uc := profileusecase.NewProfileUsecase(userInfoMock{}, profileRepositoryMock{posts: map[uuid.UUID][]models.Post{profileUserID: tt.posts}})
			got, err := uc.GetMyPosts(context.Background(), profileUserID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
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
			uc := profileusecase.NewProfileUsecase(userInfoMock{}, profileRepositoryMock{postsErr: tt.postsErr})
			got, err := uc.GetMyPosts(context.Background(), profileUserID)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
