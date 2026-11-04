package profileusecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	profileUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	profileTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
)

type profileRepoMock struct {
	profile     *models.ProfileReadModel
	err         error
	gotUsername *string
}

func (s profileRepoMock) GetProfileByUsername(_ context.Context, username string) (*models.ProfileReadModel, error) {
	if s.gotUsername != nil {
		*s.gotUsername = username
	}

	return s.profile, s.err
}

func (s profileRepoMock) GetProfileByID(_ context.Context, _ uuid.UUID) (*models.ProfileReadModel, error) {
	return s.profile, s.err
}

type userUpdaterMock struct {
	calls *[]string
	err   error
}

func (m userUpdaterMock) UpdateNickname(_ context.Context, _ uuid.UUID, nickname string) (string, error) {
	*m.calls = append(*m.calls, "user")
	return nickname, m.err
}

type authorUpdaterMock struct {
	calls *[]string
	err   error
}

func (m authorUpdaterMock) UpdateAuthor(_ context.Context, _ uuid.UUID, bio string, category *string) (*authordto.AuthorInfo, error) {
	*m.calls = append(*m.calls, "author")
	if m.err != nil {
		return nil, m.err
	}

	return &authordto.AuthorInfo{Bio: bio, Category: category}, nil
}

func TestProfileUsecase_GetMyProfile_Success(t *testing.T) {
	t.Parallel()

	path := "avatars/ivan.png"
	avatar := &path
	category := "Технологии"
	tests := []struct {
		name   string
		author *models.ProfileAuthorReadModel
		want   *profiledto.ProfileResponse
	}{
		{
			name:   "author profile",
			author: &models.ProfileAuthorReadModel{Bio: "Пишу о технологиях", Category: &category},
			want: &profiledto.ProfileResponse{
				User:   profiledto.ProfileUserResponse{PublicProfileUserResponse: profiledto.PublicProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", CreatedAt: profileTime}, Email: "ivan@example.test"},
				Author: &profiledto.AuthorResponse{Bio: "Пишу о технологиях", Category: &category},
			},
		},
		{
			name: "reader has no author profile",
			want: &profiledto.ProfileResponse{
				User: profiledto.ProfileUserResponse{PublicProfileUserResponse: profiledto.PublicProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", CreatedAt: profileTime}, Email: "ivan@example.test"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := profileusecase.NewProfileUsecase(nil, nil, profileRepoMock{profile: &models.ProfileReadModel{
				UserID:    profileUserID,
				Username:  "ivan001",
				Nickname:  "Иван",
				Email:     "ivan@example.test",
				AvatarKey: avatar,
				CreatedAt: profileTime,
				Status:    "active",
				Author:    tt.author,
			}})
			got, err := uc.GetMyProfile(context.Background(), profileUserID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestProfileUsecase_GetMyProfile_InactiveUser(t *testing.T) {
	t.Parallel()

	uc := profileusecase.NewProfileUsecase(nil, nil, profileRepoMock{profile: &models.ProfileReadModel{UserID: profileUserID, Status: "blocked"}})

	got, err := uc.GetMyProfile(context.Background(), profileUserID)

	require.Nil(t, got)
	require.ErrorIs(t, err, profileusecase.ErrUserInactive)
}

func TestProfileUsecase_GetMyProfile_Errors(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("profile storage unavailable")
	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "unknown user", repoErr: models.ErrProfileNotFound, wantErr: profileusecase.ErrProfileNotFound},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := profileusecase.NewProfileUsecase(nil, nil, profileRepoMock{err: tt.repoErr})
			got, err := uc.GetMyProfile(context.Background(), profileUserID)
			require.Nil(t, got)
			require.ErrorIs(t, err, tt.wantErr)
			if tt.wantErr != profileusecase.ErrProfileNotFound {
				require.Equal(t, false, errors.Is(err, profileusecase.ErrProfileNotFound))
			}
		})
	}
}

func TestProfileUsecase_GetPublicProfile(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("profile storage unavailable")
	tests := []struct {
		name    string
		profile *models.ProfileReadModel
		repoErr error
		wantErr error
	}{
		{name: "active user", profile: &models.ProfileReadModel{UserID: profileUserID, Username: "ivan001", Email: "ivan@example.test", Status: "active"}},
		{name: "blocked user is hidden", profile: &models.ProfileReadModel{Status: "blocked"}, wantErr: profileusecase.ErrProfileNotFound},
		{name: "deleted user is hidden", profile: &models.ProfileReadModel{Status: "deleted"}, wantErr: profileusecase.ErrProfileNotFound},
		{name: "unknown user", repoErr: models.ErrProfileNotFound, wantErr: profileusecase.ErrProfileNotFound},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotUsername string
			uc := profileusecase.NewProfileUsecase(nil, nil, profileRepoMock{profile: tt.profile, err: tt.repoErr, gotUsername: &gotUsername})

			got, err := uc.GetPublicProfile(context.Background(), "ivan001")

			require.Equal(t, "ivan001", gotUsername)

			if tt.wantErr != nil {
				require.Nil(t, got)
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			require.Equal(t, "ivan001", got.User.Username)
		})
	}
}

func TestProfileUsecase_UpdateMyProfile(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("storage unavailable")
	category := "Технологии"
	authorReq := &profiledto.UpdateAuthorRequest{Bio: "Новое описание", Category: &category}
	authorModel := &models.ProfileAuthorReadModel{Bio: "Старое описание"}
	tests := []struct {
		name       string
		status     string
		repoErr    error
		author     *models.ProfileAuthorReadModel
		req        *profiledto.UpdateAuthorRequest
		authorErr  error
		userErr    error
		wantCalls  []string
		wantErr    error
		wantAuthor *profiledto.AuthorResponse
	}{
		{name: "blocked user changes nothing", status: "blocked", author: authorModel, req: authorReq, wantErr: profileusecase.ErrUserInactive},
		{name: "deleted user changes nothing", status: "deleted", wantErr: profileusecase.ErrUserInactive},
		{name: "unknown profile changes nothing", repoErr: models.ErrProfileNotFound, req: authorReq, wantErr: profileusecase.ErrProfileNotFound},
		{name: "profile lookup failure changes nothing", repoErr: technicalErr, wantErr: technicalErr},
		{name: "reader sending author changes nothing", req: authorReq, wantErr: profileusecase.ErrNotAuthor},
		{name: "reader updates only user", wantCalls: []string{"user"}},
		{
			name:       "author is updated before user",
			author:     authorModel,
			req:        authorReq,
			wantCalls:  []string{"author", "user"},
			wantAuthor: &profiledto.AuthorResponse{Bio: "Новое описание", Category: &category},
		},
		{
			name:       "author without author block keeps data",
			author:     authorModel,
			wantCalls:  []string{"user"},
			wantAuthor: &profiledto.AuthorResponse{Bio: "Старое описание"},
		},
		{name: "author storage failure stops update", author: authorModel, req: authorReq, authorErr: technicalErr, wantCalls: []string{"author"}, wantErr: technicalErr},
		{name: "user storage failure after author update", author: authorModel, req: authorReq, userErr: technicalErr, wantCalls: []string{"author", "user"}, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status := tt.status
			if status == "" {
				status = "active"
			}

			var calls []string

			uc := profileusecase.NewProfileUsecase(
				userUpdaterMock{calls: &calls, err: tt.userErr},
				authorUpdaterMock{calls: &calls, err: tt.authorErr},
				profileRepoMock{
					profile: &models.ProfileReadModel{UserID: profileUserID, Nickname: "Старый ник", Status: status, Author: tt.author},
					err:     tt.repoErr,
				},
			)

			got, err := uc.UpdateMyProfile(context.Background(), profileUserID, profiledto.UpdateProfileRequest{
				Nickname: "Иван",
				Author:   tt.req,
			})

			require.Equal(t, tt.wantCalls, calls)

			if tt.wantErr != nil {
				require.Nil(t, got)
				require.ErrorIs(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
			require.Equal(t, "Иван", got.User.Nickname)
			require.Equal(t, tt.wantAuthor, got.Author)
		})
	}
}
