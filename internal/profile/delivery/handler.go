package profilehandlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/dto"
	profileusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/profile/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const (
	avatarFormField      = "avatar"
	maxAvatarSize        = 5 << 20
	maxAvatarRequestSize = maxAvatarSize + 1<<20
)

var (
	errInvalidAvatarForm = errors.New("invalid avatar form")
	errAvatarTooLarge    = errors.New("avatar too large")
)

type ProfileUsecase interface {
	GetMyProfile(ctx context.Context, userID uuid.UUID) (*dto.ProfileResponse, error)
	GetMyPosts(ctx context.Context, userID uuid.UUID) (*dto.ProfilePostsResponse, error)
	UploadAvatar(ctx context.Context, userID uuid.UUID, file io.ReadSeeker, size int64) (string, error)
	DeleteAvatar(ctx context.Context, userID uuid.UUID) error
}

type ProfileHandler struct {
	uc     ProfileUsecase
	logger *slog.Logger
}

func NewProfileHandler(uc ProfileUsecase, logger *slog.Logger) *ProfileHandler {
	return &ProfileHandler{uc: uc, logger: logger}
}

func (h *ProfileHandler) RegisterRoutes(public *mux.Router, private *mux.Router) {
	private.HandleFunc("/profile/me", h.GetMe).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/posts", h.GetMePosts).Methods(http.MethodGet, http.MethodOptions)
	private.HandleFunc("/profile/me/avatar", h.UploadAvatar).Methods(http.MethodPut, http.MethodOptions)
	private.HandleFunc("/profile/me/avatar", h.DeleteAvatar).Methods(http.MethodDelete, http.MethodOptions)
}

// GetMe возвращает профиль текущего пользователя
// @Summary Получение профиля текущего пользователя
// @Description Возвращает данные пользователя и информацию о том, является ли он автором
// @Tags profile
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.ProfileResponse "Профиль пользователя"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [get]
func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok || id == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	profile, err := h.uc.GetMyProfile(r.Context(), id)
	if err != nil {
		h.handleError(r.Context(), w, "get my profile", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, profile)
}

// GetMePosts возвращает посты текущего пользователя
// @Summary Получение постов текущего пользователя
// @Description Возвращает посты текущего пользователя; если постов нет, возвращает пустой список
// @Tags profile
// @Produce json
// @Security CookieAuth
// @Success 200 {object} dto.ProfilePostsResponse "Посты пользователя"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/posts [get]
func (h *ProfileHandler) GetMePosts(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok || id == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	posts, err := h.uc.GetMyPosts(r.Context(), id)
	if err != nil {
		h.handleError(r.Context(), w, "get my posts", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, posts)
}

// UploadAvatar загружает аватар текущего пользователя
// @Summary Загрузка аватара текущего пользователя
// @Description Принимает изображение jpeg, png или webp до 5 МБ в поле avatar и заменяет им текущий аватар. Формат определяется по содержимому файла
// @Tags profile
// @Accept mpfd
// @Produce json
// @Security CookieAuth
// @Param avatar formData file true "Изображение аватара"
// @Success 200 {object} dto.AvatarResponse "Аватар загружен"
// @Failure 400 {object} response.ErrorResponse "Запрос не multipart/form-data или в нем нет файла"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 413 {object} response.ErrorResponse "Файл больше 5 МБ"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 415 {object} response.ErrorResponse "Неподдерживаемый формат изображения"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/avatar [put]
func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok || id == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarRequestSize)
	if err := r.ParseMultipartForm(maxAvatarRequestSize); err != nil {
		h.handleError(r.Context(), w, "upload avatar", fmt.Errorf("%w: %w", errInvalidAvatarForm, err))
		return
	}

	file, header, err := r.FormFile(avatarFormField)
	if err != nil {
		h.handleError(r.Context(), w, "upload avatar", fmt.Errorf("%w: %w", errInvalidAvatarForm, err))
		return
	}
	defer file.Close()

	if header.Size == 0 {
		h.handleError(r.Context(), w, "upload avatar", fmt.Errorf("%w: empty file", errInvalidAvatarForm))
		return
	}
	if header.Size > maxAvatarSize {
		h.handleError(r.Context(), w, "upload avatar", errAvatarTooLarge)
		return
	}

	avatarURL, err := h.uc.UploadAvatar(r.Context(), id, file, header.Size)
	if err != nil {
		h.handleError(r.Context(), w, "upload avatar", err)
		return
	}
	response.WriteJSON(w, http.StatusOK, dto.ToAvatarResponse(avatarURL))
}

// DeleteAvatar удаляет аватар текущего пользователя
// @Summary Удаление аватара текущего пользователя
// @Description Удаляет текущий аватар. Если аватара нет, тоже возвращает 204
// @Tags profile
// @Security CookieAuth
// @Success 204 "Аватар удален"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/avatar [delete]
func (h *ProfileHandler) DeleteAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := middleware.UserIDFromContext(r.Context())
	if !ok || id == uuid.Nil {
		response.ErrorUnauthorized(w)
		return
	}

	if err := h.uc.DeleteAvatar(r.Context(), id); err != nil {
		h.handleError(r.Context(), w, "delete avatar", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) handleError(ctx context.Context, w http.ResponseWriter, handler string, err error) {
	h.logger.ErrorContext(ctx, "profile error",
		slog.String("handler", handler),
		slog.String("error", err.Error()),
	)

	var maxBytesErr *http.MaxBytesError
	switch {
	case errors.Is(err, profileusecase.ErrProfileNotFound):
		response.ErrorNotFound(w)
	case errors.As(err, &maxBytesErr), errors.Is(err, errAvatarTooLarge):
		response.ErrorPayloadTooLarge(w)
	case errors.Is(err, profileusecase.ErrUnsupportedAvatarType):
		response.ErrorUnsupportedMediaType(w)
	case errors.Is(err, errInvalidAvatarForm):
		response.ErrorBadRequest(w)
	default:
		response.ErrorInternal(w)
	}
}
