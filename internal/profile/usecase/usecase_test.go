package profileusecase_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase/mock_profile_usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	profileUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	profileTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	testLogger    = slog.New(slog.NewTextHandler(io.Discard, nil))
)

const testAvatarBaseURL = "http://localhost:9000/media"

func TestProfileUsecase_GetMyProfile_Success(t *testing.T) {
	t.Parallel()

	path := "avatars/ivan.png"
	avatar := &path
	url := "http://localhost:9000/media/avatars/ivan.png"
	avatarURL := &url
	category := "Технологии"
	tests := []struct {
		name   string
		author *models.ProfileAuthor
		want   *profiledto.ProfileResponse
	}{
		{
			name:   "author profile",
			author: &models.ProfileAuthor{Bio: "Пишу о технологиях", Category: &category},
			want: &profiledto.ProfileResponse{
				User:   profiledto.ProfileUserResponse{PublicProfileUserResponse: profiledto.PublicProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", AvatarURL: avatarURL, CreatedAt: profileTime}, Email: "ivan@example.test"},
				Author: &profiledto.AuthorResponse{Bio: "Пишу о технологиях", Category: &category},
			},
		},
		{
			name: "reader has no author profile",
			want: &profiledto.ProfileResponse{
				User: profiledto.ProfileUserResponse{PublicProfileUserResponse: profiledto.PublicProfileUserResponse{ID: profileUserID, Username: "ivan001", Nickname: "Иван", AvatarURL: avatarURL, CreatedAt: profileTime}, Email: "ivan@example.test"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_profile_usecase.NewMockProfileRepository(t)
			repo.EXPECT().GetProfileByID(mock.Anything, profileUserID).Return(&models.Profile{
				UserID:    profileUserID,
				Username:  "ivan001",
				Nickname:  "Иван",
				Email:     "ivan@example.test",
				AvatarKey: avatar,
				CreatedAt: profileTime,
				Status:    "active",
				Author:    tt.author,
			}, nil)

			uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBaseURL, testLogger)
			got, err := uc.GetMyProfile(context.Background(), profileUserID)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestProfileUsecase_GetMyProfile_InactiveUser(t *testing.T) {
	t.Parallel()

	repo := mock_profile_usecase.NewMockProfileRepository(t)
	repo.EXPECT().GetProfileByID(mock.Anything, profileUserID).Return(&models.Profile{UserID: profileUserID, Status: "blocked"}, nil)

	uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBaseURL, testLogger)

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

			repo := mock_profile_usecase.NewMockProfileRepository(t)
			repo.EXPECT().GetProfileByID(mock.Anything, profileUserID).Return(nil, tt.repoErr)

			uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBaseURL, testLogger)
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
		profile *models.Profile
		repoErr error
		wantErr error
	}{
		{name: "active user", profile: &models.Profile{UserID: profileUserID, Username: "ivan001", Email: "ivan@example.test", Status: "active"}},
		{name: "blocked user is hidden", profile: &models.Profile{Status: "blocked"}, wantErr: profileusecase.ErrProfileNotFound},
		{name: "deleted user is hidden", profile: &models.Profile{Status: "deleted"}, wantErr: profileusecase.ErrProfileNotFound},
		{name: "unknown user", repoErr: models.ErrProfileNotFound, wantErr: profileusecase.ErrProfileNotFound},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_profile_usecase.NewMockProfileRepository(t)
			repo.EXPECT().GetProfileByUsername(mock.Anything, "ivan001").Return(tt.profile, tt.repoErr)

			uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBaseURL, testLogger)

			got, err := uc.GetPublicProfile(context.Background(), "ivan001")

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
	authorModel := &models.ProfileAuthor{Bio: "Старое описание"}
	newAuthor := &authordto.AuthorInfo{Bio: "Новое описание", Category: &category}
	oldAuthor := &authordto.AuthorInfo{Bio: "Старое описание"}
	tests := []struct {
		name       string
		status     string
		repoErr    error
		author     *models.ProfileAuthor
		req        *profiledto.UpdateAuthorRequest
		setupMocks func(u *mock_profile_usecase.MockUserUsecase, a *mock_profile_usecase.MockAuthorUsecase)
		wantErr    error
		wantAuthor *profiledto.AuthorResponse
	}{
		{name: "blocked user changes nothing", status: "blocked", author: authorModel, req: authorReq, wantErr: profileusecase.ErrUserInactive},
		{name: "deleted user changes nothing", status: "deleted", wantErr: profileusecase.ErrUserInactive},
		{name: "unknown profile changes nothing", repoErr: models.ErrProfileNotFound, req: authorReq, wantErr: profileusecase.ErrProfileNotFound},
		{name: "profile lookup failure changes nothing", repoErr: technicalErr, wantErr: technicalErr},
		{name: "reader sending author changes nothing", req: authorReq, wantErr: profileusecase.ErrNotAuthor},
		{
			name: "reader updates only user",
			setupMocks: func(u *mock_profile_usecase.MockUserUsecase, a *mock_profile_usecase.MockAuthorUsecase) {
				u.EXPECT().UpdateNickname(mock.Anything, profileUserID, "Иван").Return("Иван", nil)
			},
		},
		{
			name:   "author is updated before user",
			author: authorModel,
			req:    authorReq,
			setupMocks: func(u *mock_profile_usecase.MockUserUsecase, a *mock_profile_usecase.MockAuthorUsecase) {
				mock.InOrder(
					a.EXPECT().UpdateAuthor(mock.Anything, authordto.UpdateAuthorRequest{AuthorID: profileUserID, Bio: "Новое описание", Category: &category}).Return(newAuthor, nil).Call,
					u.EXPECT().UpdateNickname(mock.Anything, profileUserID, "Иван").Return("Иван", nil).Call,
				)
			},
			wantAuthor: &profiledto.AuthorResponse{Bio: "Новое описание", Category: &category},
		},
		{
			name:   "author without author block keeps data",
			author: authorModel,
			setupMocks: func(u *mock_profile_usecase.MockUserUsecase, a *mock_profile_usecase.MockAuthorUsecase) {
				u.EXPECT().UpdateNickname(mock.Anything, profileUserID, "Иван").Return("Иван", nil)
			},
			wantAuthor: &profiledto.AuthorResponse{Bio: "Старое описание"},
		},
		{
			name:   "author storage failure stops update",
			author: authorModel,
			req:    authorReq,
			setupMocks: func(u *mock_profile_usecase.MockUserUsecase, a *mock_profile_usecase.MockAuthorUsecase) {
				a.EXPECT().UpdateAuthor(mock.Anything, authordto.UpdateAuthorRequest{AuthorID: profileUserID, Bio: "Новое описание", Category: &category}).Return(nil, technicalErr)
			},
			wantErr: technicalErr,
		},
		{
			name:   "user storage failure after author update",
			author: authorModel,
			req:    authorReq,
			setupMocks: func(u *mock_profile_usecase.MockUserUsecase, a *mock_profile_usecase.MockAuthorUsecase) {
				mock.InOrder(
					a.EXPECT().UpdateAuthor(mock.Anything, authordto.UpdateAuthorRequest{AuthorID: profileUserID, Bio: "Новое описание", Category: &category}).Return(newAuthor, nil).Call,
					u.EXPECT().UpdateNickname(mock.Anything, profileUserID, "Иван").Return("", technicalErr).Call,
					a.EXPECT().UpdateAuthor(mock.Anything, authordto.UpdateAuthorRequest{AuthorID: profileUserID, Bio: "Старое описание"}).Return(oldAuthor, nil).Call,
				)
			},
			wantErr: technicalErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status := tt.status
			if status == "" {
				status = "active"
			}

			users := mock_profile_usecase.NewMockUserUsecase(t)
			authors := mock_profile_usecase.NewMockAuthorUsecase(t)
			if tt.setupMocks != nil {
				tt.setupMocks(users, authors)
			}

			repo := mock_profile_usecase.NewMockProfileRepository(t)
			repo.EXPECT().GetProfileByID(mock.Anything, profileUserID).Return(
				&models.Profile{UserID: profileUserID, Nickname: "Старый ник", Status: status, Author: tt.author},
				tt.repoErr,
			)

			uc := profileusecase.NewProfileUsecase(users, authors, repo, nil, testAvatarBaseURL, testLogger)

			got, err := uc.UpdateMyProfile(context.Background(), profileUserID, profiledto.UpdateProfileRequest{
				Nickname: "Иван",
				Author:   tt.req,
			})

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

func TestProfileUsecase_UploadAvatar_TooLarge(t *testing.T) {
	t.Parallel()

	uc := profileusecase.NewProfileUsecase(nil, nil, nil, nil, testAvatarBaseURL, testLogger)

	got, err := uc.UploadAvatar(context.Background(), profileUserID, bytes.NewReader(nil), profileusecase.MaxAvatarSize+1)

	require.Empty(t, got)
	require.ErrorIs(t, err, profileusecase.ErrAvatarTooLarge)
}
