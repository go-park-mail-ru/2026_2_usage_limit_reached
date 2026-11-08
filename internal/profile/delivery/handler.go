package profilehandlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	avatarFormField      = "avatar"
	maxAvatarRequestSize = profileusecase.MaxAvatarSize + 1<<20
)

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	GetPublicProfile(ctx context.Context, username string) (*dto.PublicProfileResponse, error)
	UpdateMyProfile(ctx context.Context, userID uuid.UUID, req dto.UpdateProfileRequest) (*dto.ProfileResponse, error)
	BecomeAuthor(ctx context.Context, userID uuid.UUID, req dto.UpdateAuthorRequest) (*dto.ProfileResponse, error)
	UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID) error
}

type ProfileHandler struct {
	uc        ProfileUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewProfileHandler(uc ProfileUsecase, validator *validator.Validator, logger *slog.Logger) *ProfileHandler {
	return &ProfileHandler{uc: uc, validator: validator, logger: logger}
}

func (h *ProfileHandler) RegisterRoutes(public *mux.Router, private *mux.Router) {
	private.HandleFunc("/profile/me", h.GetMe).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me", h.UpdateMe).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/profile/me/author", h.BecomeAuthor).Methods(http.MethodPost, http.MethodOptions)
	private.HandleFunc("/profile/me/avatar", h.UploadAvatar).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/profile/me/avatar", h.DeleteAvatar).Methods(http.MethodDelete, http.MethodOptions)
	public.HandleFunc("/users/{username}", h.GetByUsername).Methods(http.MethodGet, http.MethodOptions)
}

// GetMe возвращает профиль текущего пользователя
// @Summary Получение профиля текущего пользователя
// @Description Возвращает данные пользователя. Блок author есть только у автора, avatar_url - только при наличии аватара
// @Tags profile
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.ProfileResponse "Профиль пользователя"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь заблокирован или удалён"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [get]
func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserInfoFromContext(r.Context())
	if !ok || user == nil || user.UserID == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	profile, err := h.uc.GetMyProfile(r.Context(), user.UserID)
	if err != nil {
		h.handleError(r.Context(), w, "get my profile", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, profile)
}

// GetByUsername возвращает публичный профиль пользователя
// @Summary Получение публичного профиля пользователя
// @Description Возвращает публичные данные пользователя без email. Блок author есть только у автора, avatar_url - только при наличии аватара
// @Tags profile
// @Produce json
// @Param username path string true "Username пользователя"
// @Success 200 {object} dto.PublicProfileResponse "Публичный профиль пользователя"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден, заблокирован или удалён"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /users/{username} [get]
func (h *ProfileHandler) GetByUsername(w http.ResponseWriter, r *http.Request) {
	profile, err := h.uc.GetPublicProfile(r.Context(), mux.Vars(r)["username"])
	if err != nil {
		h.handleError(r.Context(), w, "get public profile", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// UpdateMe изменяет профиль текущего пользователя
// @Summary Изменение профиля текущего пользователя
// @Description Сохраняет nickname и, если передан блок author, bio и category. Если category нет или она null, категория очищается
// @Tags profile
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param request body dto.UpdateProfileRequest true "Новые данные профиля"
// @Success 200 {object} dto.ProfileResponse "Профиль изменен"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь не является автором, заблокирован или удалён, либо неверный CSRF-токен или его отсутствие"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [put]
func (h *ProfileHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserInfoFromContext(r.Context())
	if !ok || user == nil || user.UserID == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.UpdateProfileRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		h.handleError(r.Context(), w, "update my profile", fmt.Errorf("%w: %v", validator.ErrValidation, err))
		return
	}

	if err := req.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "update my profile", err)
		return
	}

	profile, err := h.uc.UpdateMyProfile(r.Context(), user.UserID, req)
	if err != nil {
		h.handleError(r.Context(), w, "update my profile", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// BecomeAuthor создает профиль автора текущего пользователя
// @Summary Создание профиля автора
// @Description Создает профиль автора. Если профиль автора уже есть, возвращает его без изменений, переданные данные игнорируются. Если category не выбрана, поле опускается
// @Tags profile
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param request body internal_profile_dto.UpdateAuthorRequest true "Данные профиля автора"
// @Success 200 {object} dto.ProfileResponse "Профиль с блоком author"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь заблокирован или удалён, либо неверный CSRF-токен или его отсутствие"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/author [post]
func (h *ProfileHandler) BecomeAuthor(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserInfoFromContext(r.Context())
	if !ok || user == nil || user.UserID == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.UpdateAuthorRequest
	if err := response.DecodeJSON(w, r, &req); err != nil {
		h.handleError(r.Context(), w, "become author", fmt.Errorf("%w: %v", validator.ErrValidation, err))
		return
	}

	if err := req.Validate(h.validator); err != nil {
		h.handleError(r.Context(), w, "become author", err)
		return
	}

	profile, err := h.uc.BecomeAuthor(r.Context(), user.UserID, req)
	if err != nil {
		h.handleError(r.Context(), w, "become author", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// UploadAvatar загружает аватар текущего пользователя
// @Summary Загрузка аватара текущего пользователя
// @Description Принимает изображение jpeg, png или webp до 5 МБ в поле avatar и заменяет им текущий аватар. Формат определяется по содержимому файла
// @Tags profile
// @Accept mpfd
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param avatar formData file true "Изображение аватара"
// @Success 200 {object} dto.AvatarResponse "Аватар загружен"
// @Failure 400 {object} response.ErrorResponse "Запрос не multipart/form-data или в нем нет файла"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 413 {object} response.ErrorResponse "Файл больше 5 МБ"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 415 {object} response.ErrorResponse "Неподдерживаемый формат изображения"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/avatar [put]
func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok || user == nil || user.UserID == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	file, size, err := readAvatar(w, r)
	if err != nil {
		h.handleError(ctx, w, "upload avatar", err)
		return
	}
	defer file.Close()

	avatarURL, err := h.uc.UploadAvatar(ctx, user.UserID, file, size)
	if err != nil {
		h.handleError(ctx, w, "upload avatar", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, dto.ToAvatarResponse(avatarURL))
}

func readAvatar(w http.ResponseWriter, r *http.Request) (multipart.File, int64, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarRequestSize)
	if err := r.ParseMultipartForm(maxAvatarRequestSize); err != nil {
		var maxBytesErr *http.MaxBytesError

		if errors.As(err, &maxBytesErr) {
			return nil, 0, fmt.Errorf("%w: %w", profileusecase.ErrAvatarTooLarge, err)
		}

		return nil, 0, fmt.Errorf("%w: %w", validator.ErrValidation, err)
	}

	file, header, err := r.FormFile(avatarFormField)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %w", validator.ErrValidation, err)
	}

	if header.Size == 0 {
		file.Close()
		return nil, 0, fmt.Errorf("%w: empty avatar file", validator.ErrValidation)
	}

	return file, header.Size, nil
}

// DeleteAvatar удаляет аватар текущего пользователя
// @Summary Удаление аватара текущего пользователя
// @Description Удаляет текущий аватар. Если аватара нет, тоже возвращает 204
// @Tags profile
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Success 204 "Аватар удален"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Неверный CSRF-токен или его отсутствие"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/avatar [delete]
func (h *ProfileHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok || user == nil || user.UserID == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	if err := h.uc.DeleteAvatar(ctx, user.UserID); err != nil {
		h.handleError(ctx, w, "delete avatar", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) handleError(ctx context.Context, w http.ResponseWriter, handler string, err error) {
	level := slog.LevelInfo

	switch {
	case errors.Is(err, profileusecase.ErrProfileNotFound):
		response.ErrorNotFound(w)
	case errors.Is(err, profileusecase.ErrAvatarTooLarge):
		response.ErrorPayloadTooLarge(w)
	case errors.Is(err, profileusecase.ErrUnsupportedAvatarType):
		response.ErrorUnsupportedMediaType(w)
	case errors.Is(err, validator.ErrValidation):
		response.ErrorBadRequest(w)
	case errors.Is(err, profileusecase.ErrNotAuthor), errors.Is(err, profileusecase.ErrUserInactive):
		response.ErrorForbidden(w)
	default:
		level = slog.LevelError

		response.ErrorInternal(w)
	}

	h.logger.Log(ctx, level, "profile error",
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)
}
