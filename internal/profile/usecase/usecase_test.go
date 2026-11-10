package profileusecase_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	authordto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/author/dto"
	profiledto "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/models"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase/mock_profile_usecase"
	userusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/user/usecase"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	profileUserID = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	profileTime   = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	testLogger    = slog.New(slog.NewTextHandler(io.Discard, nil))
)

const (
	testAvatarBasePath = "/media"
	// Минимальное webp-изображение 1x1: стандартная библиотека умеет только декодировать webp.
	webpAvatarBase64 = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="
)

func TestProfileUsecase_GetMyProfile_Success(t *testing.T) {
	t.Parallel()

	path := "avatars/ivan.png"
	avatar := &path
	url := "/media/avatars/ivan.png"
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

			uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBasePath, testLogger)
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

	uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBasePath, testLogger)

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

			uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBasePath, testLogger)
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

			uc := profileusecase.NewProfileUsecase(nil, nil, repo, nil, testAvatarBasePath, testLogger)

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

			uc := profileusecase.NewProfileUsecase(users, authors, repo, nil, testAvatarBasePath, testLogger)

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

	uc := profileusecase.NewProfileUsecase(nil, nil, nil, nil, testAvatarBasePath, testLogger)

	got, err := uc.UploadAvatar(context.Background(), profileUserID, bytes.NewReader(nil), profileusecase.MaxAvatarSize+1)

	require.Empty(t, got)
	require.ErrorIs(t, err, profileusecase.ErrAvatarTooLarge)
}

func TestProfileUsecase_UpdateMyProfile_RollbackFailure(t *testing.T) {
	t.Parallel()

	userErr := errors.New("user storage unavailable")
	rollbackErr := errors.New("author storage unavailable")
	users := mock_profile_usecase.NewMockUserUsecase(t)
	authors := mock_profile_usecase.NewMockAuthorUsecase(t)
	repo := mock_profile_usecase.NewMockProfileRepository(t)
	repo.EXPECT().GetProfileByID(mock.Anything, profileUserID).Return(
		&models.Profile{UserID: profileUserID, Status: "active", Author: &models.ProfileAuthor{Bio: "Старое описание"}},
		nil,
	)
	mock.InOrder(
		authors.EXPECT().UpdateAuthor(mock.Anything, authordto.UpdateAuthorRequest{AuthorID: profileUserID, Bio: "Новое описание"}).Return(&authordto.AuthorInfo{Bio: "Новое описание"}, nil).Call,
		users.EXPECT().UpdateNickname(mock.Anything, profileUserID, "Иван").Return("", userErr).Call,
		authors.EXPECT().UpdateAuthor(mock.Anything, authordto.UpdateAuthorRequest{AuthorID: profileUserID, Bio: "Старое описание"}).Return(nil, rollbackErr).Call,
	)
	uc := profileusecase.NewProfileUsecase(users, authors, repo, nil, testAvatarBasePath, testLogger)

	got, err := uc.UpdateMyProfile(context.Background(), profileUserID, profiledto.UpdateProfileRequest{
		Nickname: "Иван",
		Author:   &profiledto.UpdateAuthorRequest{Bio: "Новое описание"},
	})

	require.Nil(t, got)
	require.ErrorIs(t, err, userErr)
	require.ErrorIs(t, err, rollbackErr)
}

func pngAvatar(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1))))

	return buf.Bytes()
}

func jpegAvatar(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 1, 1)), nil))

	return buf.Bytes()
}

func webpAvatar(t *testing.T) []byte {
	t.Helper()

	data, err := base64.StdEncoding.DecodeString(webpAvatarBase64)
	require.NoError(t, err)

	return data
}

// avatarKey сопоставляет ключ аватара пользователя с заданным расширением, не завися от случайного uuid в имени
func avatarKey(ext string) any {
	prefix := "avatars/" + profileUserID.String() + "/"

	return mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, prefix) && strings.HasSuffix(key, ext)
	})
}

func avatarKeyPtr() any {
	prefix := "avatars/" + profileUserID.String() + "/"

	return mock.MatchedBy(func(key *string) bool {
		return key != nil && strings.HasPrefix(*key, prefix)
	})
}

// unseekableFile имитирует файл, который нельзя перемотать на начало после определения формата
type unseekableFile struct {
	io.Reader
	seekErr error
}

func (f unseekableFile) Seek(int64, int) (int64, error) {
	return 0, f.seekErr
}

func TestProfileUsecase_UploadAvatar_Formats(t *testing.T) {
	tests := []struct {
		name        string
		image       []byte
		ext         string
		contentType string
	}{
		{name: "png", image: pngAvatar(t), ext: ".png", contentType: "image/png"},
		{name: "jpeg", image: jpegAvatar(t), ext: ".jpg", contentType: "image/jpeg"},
		{name: "webp", image: webpAvatar(t), ext: ".webp", contentType: "image/webp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := int64(len(tt.image))
			users := mock_profile_usecase.NewMockUserUsecase(t)
			users.EXPECT().UpdateAvatarKey(mock.Anything, profileUserID, avatarKeyPtr()).Return(nil, nil)
			storage := mock_profile_usecase.NewMockAvatarStorage(t)
			storage.EXPECT().Put(mock.Anything, avatarKey(tt.ext), mock.Anything, size, tt.contentType).
				RunAndReturn(func(_ context.Context, _ string, r io.Reader, _ int64, _ string) error {
					uploaded, err := io.ReadAll(r)
					require.NoError(t, err)
					require.Equal(t, tt.image, uploaded, "файл должен быть перемотан на начало перед загрузкой")

					return nil
				})
			uc := profileusecase.NewProfileUsecase(users, nil, nil, storage, testAvatarBasePath, testLogger)

			got, err := uc.UploadAvatar(context.Background(), profileUserID, bytes.NewReader(tt.image), size)

			require.NoError(t, err)
			require.True(t, strings.HasPrefix(got, testAvatarBasePath+"/avatars/"+profileUserID.String()+"/"), got)
			require.True(t, strings.HasSuffix(got, tt.ext), got)
		})
	}
}

func TestProfileUsecase_UploadAvatar_ReplacesOldAvatar(t *testing.T) {
	oldKey := "avatars/old.png"
	tests := []struct {
		name      string
		deleteErr error
	}{
		{name: "old file is removed"},
		{name: "old file removal failure does not fail upload", deleteErr: errors.New("storage unavailable")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			avatar := pngAvatar(t)
			users := mock_profile_usecase.NewMockUserUsecase(t)
			users.EXPECT().UpdateAvatarKey(mock.Anything, profileUserID, avatarKeyPtr()).Return(&oldKey, nil)
			storage := mock_profile_usecase.NewMockAvatarStorage(t)
			storage.EXPECT().Put(mock.Anything, avatarKey(".png"), mock.Anything, int64(len(avatar)), "image/png").Return(nil)
			storage.EXPECT().Delete(mock.Anything, oldKey).Return(tt.deleteErr)
			uc := profileusecase.NewProfileUsecase(users, nil, nil, storage, testAvatarBasePath, testLogger)

			got, err := uc.UploadAvatar(context.Background(), profileUserID, bytes.NewReader(avatar), int64(len(avatar)))

			require.NoError(t, err)
			require.NotEmpty(t, got)
		})
	}
}

func TestProfileUsecase_UploadAvatar_UnsupportedType(t *testing.T) {
	tests := []struct {
		name string
		file []byte
	}{
		{name: "not an image", file: []byte("not an image")},
		{name: "empty file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := profileusecase.NewProfileUsecase(nil, nil, nil, nil, testAvatarBasePath, testLogger)

			got, err := uc.UploadAvatar(context.Background(), profileUserID, bytes.NewReader(tt.file), int64(len(tt.file)))

			require.Empty(t, got)
			require.ErrorIs(t, err, profileusecase.ErrUnsupportedAvatarType)
		})
	}
}

func TestProfileUsecase_UploadAvatar_RewindFailure(t *testing.T) {
	t.Parallel()

	seekErr := errors.New("seek failed")
	avatar := pngAvatar(t)
	file := unseekableFile{Reader: bytes.NewReader(avatar), seekErr: seekErr}
	uc := profileusecase.NewProfileUsecase(nil, nil, nil, nil, testAvatarBasePath, testLogger)

	got, err := uc.UploadAvatar(context.Background(), profileUserID, file, int64(len(avatar)))

	require.Empty(t, got)
	require.ErrorIs(t, err, seekErr)
}

func TestProfileUsecase_UploadAvatar_StorageFailure(t *testing.T) {
	t.Parallel()

	storageErr := errors.New("storage unavailable")
	avatar := pngAvatar(t)
	storage := mock_profile_usecase.NewMockAvatarStorage(t)
	storage.EXPECT().Put(mock.Anything, avatarKey(".png"), mock.Anything, int64(len(avatar)), "image/png").Return(storageErr)
	uc := profileusecase.NewProfileUsecase(mock_profile_usecase.NewMockUserUsecase(t), nil, nil, storage, testAvatarBasePath, testLogger)

	got, err := uc.UploadAvatar(context.Background(), profileUserID, bytes.NewReader(avatar), int64(len(avatar)))

	require.Empty(t, got)
	require.ErrorIs(t, err, storageErr)
}

func TestProfileUsecase_UploadAvatar_SetKeyFailureRemovesUploadedFile(t *testing.T) {
	technicalErr := errors.New("user storage unavailable")
	deleteErr := errors.New("storage unavailable")
	tests := []struct {
		name      string
		usersErr  error
		deleteErr error
		wantErrs  []error
	}{
		{name: "unknown user", usersErr: userusecase.ErrUserNotFound, wantErrs: []error{profileusecase.ErrProfileNotFound}},
		{name: "user storage failure", usersErr: technicalErr, wantErrs: []error{technicalErr}},
		{name: "uploaded file removal failure is reported too", usersErr: technicalErr, deleteErr: deleteErr, wantErrs: []error{technicalErr, deleteErr}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			avatar := pngAvatar(t)
			users := mock_profile_usecase.NewMockUserUsecase(t)
			users.EXPECT().UpdateAvatarKey(mock.Anything, profileUserID, avatarKeyPtr()).Return(nil, tt.usersErr)
			storage := mock_profile_usecase.NewMockAvatarStorage(t)
			storage.EXPECT().Put(mock.Anything, avatarKey(".png"), mock.Anything, int64(len(avatar)), "image/png").Return(nil)
			storage.EXPECT().Delete(mock.Anything, avatarKey(".png")).Return(tt.deleteErr)
			uc := profileusecase.NewProfileUsecase(users, nil, nil, storage, testAvatarBasePath, testLogger)

			got, err := uc.UploadAvatar(context.Background(), profileUserID, bytes.NewReader(avatar), int64(len(avatar)))

			require.Empty(t, got)
			for _, want := range tt.wantErrs {
				require.ErrorIs(t, err, want)
			}
		})
	}
}

func TestProfileUsecase_DeleteAvatar(t *testing.T) {
	oldKey := "avatars/old.png"
	tests := []struct {
		name   string
		oldKey *string
	}{
		{name: "old file is removed", oldKey: &oldKey},
		{name: "user had no avatar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mock_profile_usecase.NewMockUserUsecase(t)
			users.EXPECT().UpdateAvatarKey(mock.Anything, profileUserID, (*string)(nil)).Return(tt.oldKey, nil)
			storage := mock_profile_usecase.NewMockAvatarStorage(t)
			if tt.oldKey != nil {
				storage.EXPECT().Delete(mock.Anything, *tt.oldKey).Return(nil)
			}
			uc := profileusecase.NewProfileUsecase(users, nil, nil, storage, testAvatarBasePath, testLogger)

			require.NoError(t, uc.DeleteAvatar(context.Background(), profileUserID))
		})
	}
}

func TestProfileUsecase_DeleteAvatar_OldFileRemovalFailureIsLogged(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer

	oldKey := "avatars/old.png"
	users := mock_profile_usecase.NewMockUserUsecase(t)
	users.EXPECT().UpdateAvatarKey(mock.Anything, profileUserID, (*string)(nil)).Return(&oldKey, nil)
	storage := mock_profile_usecase.NewMockAvatarStorage(t)
	storage.EXPECT().Delete(mock.Anything, oldKey).Return(errors.New("storage unavailable"))
	uc := profileusecase.NewProfileUsecase(users, nil, nil, storage, testAvatarBasePath, slog.New(slog.NewTextHandler(&logs, nil)))

	err := uc.DeleteAvatar(context.Background(), profileUserID)

	require.NoError(t, err)
	require.Contains(t, logs.String(), "delete old avatar error")
	require.Contains(t, logs.String(), oldKey)
}

func TestProfileUsecase_DeleteAvatar_Errors(t *testing.T) {
	technicalErr := errors.New("user storage unavailable")
	tests := []struct {
		name     string
		usersErr error
		wantErr  error
	}{
		{name: "unknown user", usersErr: userusecase.ErrUserNotFound, wantErr: profileusecase.ErrProfileNotFound},
		{name: "user storage failure", usersErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := mock_profile_usecase.NewMockUserUsecase(t)
			users.EXPECT().UpdateAvatarKey(mock.Anything, profileUserID, (*string)(nil)).Return(nil, tt.usersErr)
			uc := profileusecase.NewProfileUsecase(users, nil, nil, mock_profile_usecase.NewMockAvatarStorage(t), testAvatarBasePath, testLogger)

			err := uc.DeleteAvatar(context.Background(), profileUserID)

			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}
