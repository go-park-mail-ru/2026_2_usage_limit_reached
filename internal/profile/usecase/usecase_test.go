package profileusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	authorusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/usecase"
	postdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	userdto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/dto"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	profileUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	profilePostID = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	profileTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

type userInfoMock struct {
	users map[uuid.UUID]userdto.UserInfo
	err   error
}

func (s userInfoMock) FindUserByID(_ context.Context, id uuid.UUID) (*userdto.UserInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	user, ok := s.users[id]
	if !ok {
		return nil, userusecase.ErrUserNotFound
	}
	return &user, nil
}

type authorInfoMock struct {
	authors map[uuid.UUID]authordto.AuthorInfo
	err     error
}

func (s authorInfoMock) FindAuthorByID(_ context.Context, id uuid.UUID) (*authordto.AuthorInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	author, ok := s.authors[id]
	if !ok {
		return nil, authorusecase.ErrAuthorNotFound
	}
	return &author, nil
}

type postsMock struct {
	posts map[uuid.UUID][]*postdto.PostInfo
	err   error
}

func (s postsMock) ListPostsByAuthor(_ context.Context, id uuid.UUID) ([]*postdto.PostInfo, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.posts[id], nil
}

func TestProfileUsecase_GetMyProfile_Success(t *testing.T) {
	t.Parallel()

	path := "avatars/ivan.png"
	avatar := &path
	category := "Технологии"
	user := &userdto.UserInfo{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: avatar, CreatedAt: profileTime}
	author := &authordto.AuthorInfo{ID: profileUserID, Bio: "Пишу о технологиях", Category: &category}
	tests := []struct {
		name   string
		author *authordto.AuthorInfo
		want   *profiledto.ProfileResponse
	}{
		{
			name:   "author profile",
			author: author,
			want: &profiledto.ProfileResponse{
				User:   profiledto.ProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: avatar, CreatedAt: profileTime},
				Author: &profiledto.AuthorResponse{Bio: "Пишу о технологиях", Category: "Технологии"},
			},
		},
		{
			name: "reader has no author profile",
			want: &profiledto.ProfileResponse{
				User: profiledto.ProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", Email: "ivan@example.test", AvatarKey: avatar, CreatedAt: profileTime},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authors := make(map[uuid.UUID]authordto.AuthorInfo)
			if tt.author != nil {
				authors[profileUserID] = *tt.author
			}
			uc := profileusecase.NewProfileUsecase(
				userInfoMock{users: map[uuid.UUID]userdto.UserInfo{profileUserID: *user}},
				authorInfoMock{authors: authors},
				postsMock{},
				nil,
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
		user      *userdto.UserInfo
		userErr   error
		authorErr error
		wantErr   error
	}{
		{name: "unknown user", userErr: userusecase.ErrUserNotFound, wantErr: profileusecase.ErrProfileNotFound},
		{name: "user storage failure", userErr: technicalUserErr, wantErr: technicalUserErr},
		{name: "author storage failure", user: &userdto.UserInfo{ID: profileUserID}, authorErr: technicalAuthorErr, wantErr: technicalAuthorErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			users := make(map[uuid.UUID]userdto.UserInfo)
			if tt.user != nil {
				users[profileUserID] = *tt.user
			}
			uc := profileusecase.NewProfileUsecase(userInfoMock{users: users, err: tt.userErr}, authorInfoMock{err: tt.authorErr}, postsMock{}, nil)
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

	posts := []*postdto.PostInfo{{ID: profilePostID, Author: postdto.AuthorResponse{ID: profileUserID}, Title: "Первый пост", Body: "Текст", Status: "published", PublishedAt: &profileTime, CreatedAt: profileTime}}
	tests := []struct {
		name  string
		posts []*postdto.PostInfo
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
			uc := profileusecase.NewProfileUsecase(userInfoMock{}, authorInfoMock{}, postsMock{posts: map[uuid.UUID][]*postdto.PostInfo{profileUserID: tt.posts}}, nil)
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
			uc := profileusecase.NewProfileUsecase(userInfoMock{}, authorInfoMock{}, postsMock{err: tt.postsErr}, nil)
			got, err := uc.GetMyPosts(context.Background(), profileUserID)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
