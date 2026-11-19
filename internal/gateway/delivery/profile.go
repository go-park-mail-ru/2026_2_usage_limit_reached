package handlers

import (
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/gateway/dto"
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

type ProfileHandler struct {
	identity Identity
	logger   *slog.Logger
}

func NewProfileHandler(identity Identity, logger *slog.Logger) *ProfileHandler {
	return &ProfileHandler{identity: identity, logger: logger}
}

// GetMe возвращает профиль текущего пользователя
// @Summary Получение профиля текущего пользователя
// @Description Возвращает данные пользователя. Блок author есть только у автора
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
	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	profile, err := h.identity.GetMe(r.Context(), userID)
	if err != nil {
		handleError(r.Context(), h.logger, w, "get my profile", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// GetPublicProfile возвращает публичный профиль пользователя
// @Summary Получение публичного профиля пользователя
// @Description Возвращает публичные данные пользователя без email
// @Tags profile
// @Produce json
// @Param username path string true "Username пользователя"
// @Success 200 {object} dto.PublicProfileResponse "Публичный профиль пользователя"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /users/{username} [get]
func (h *ProfileHandler) GetPublicProfile(w http.ResponseWriter, r *http.Request) {
	username := mux.Vars(r)["username"]
	if username == "" {
		logError(r.Context(), h.logger, "get public profile", errors.New("username is required"))
		response.ErrorBadRequest(w)
		return
	}

	profile, err := h.identity.GetPublicProfile(r.Context(), username)
	if err != nil {
		handleError(r.Context(), h.logger, w, "get public profile", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// UpdateMyProfile изменяет профиль текущего пользователя
// @Summary Изменение профиля текущего пользователя
// @Description Сохраняет nickname и, если передан блок author, bio и category
// @Tags profile
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param request body dto.UpdateProfileRequest true "Новые данные профиля"
// @Success 200 {object} dto.ProfileResponse "Профиль изменен"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Недостаточно прав или неверный CSRF-токен"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me [put]
func (h *ProfileHandler) UpdateMyProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.UpdateProfileRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		logError(r.Context(), h.logger, "update my profile", err)
		response.ErrorBadRequest(w)
		return
	}

	profile, err := h.identity.UpdateMyProfile(r.Context(), userID, req)
	if err != nil {
		handleError(r.Context(), h.logger, w, "update my profile", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// BecomeAuthor создает профиль автора текущего пользователя
// @Summary Создание профиля автора
// @Description Создает профиль автора. Если профиль уже есть, возвращает его без изменений
// @Tags profile
// @Accept json
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param request body dto.UpdateAuthorRequest true "Данные профиля автора"
// @Success 200 {object} dto.ProfileResponse "Профиль с блоком author"
// @Failure 400 {object} response.ErrorResponse "Ошибка валидации"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь заблокирован или удалён"
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/author [post]
func (h *ProfileHandler) BecomeAuthor(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	var req dto.UpdateAuthorRequest
	defer r.Body.Close()

	if err := response.DecodeJSON(w, r, &req); err != nil {
		logError(r.Context(), h.logger, "become author", err)
		response.ErrorBadRequest(w)
		return
	}

	profile, err := h.identity.BecomeAuthor(r.Context(), userID, req)
	if err != nil {
		handleError(r.Context(), h.logger, w, "become author", err)
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

// UploadAvatar загружает аватар текущего пользователя
// @Summary Загрузка аватара текущего пользователя
// @Description Принимает изображение jpeg, png или webp до 5 МБ в поле avatar
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
// @Failure 404 {object} response.ErrorResponse "Пользователь не найден"
// @Failure 413 {object} response.ErrorResponse "Файл больше 5 МБ"
// @Failure 415 {object} response.ErrorResponse "Неподдерживаемый формат изображения"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /profile/me/avatar [put]
func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarRequestSize)

	reader, err := r.MultipartReader()
	if err != nil {
		logError(r.Context(), h.logger, "upload avatar", err)
		response.ErrorBadRequest(w)
		return
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			h.avatarReadError(r, w, err)
			return
		}

		if part.FormName() != avatarFormField {
			_ = part.Close()
			continue
		}

		file := &readErrorRecorder{reader: part}
		avatarURL, err := h.identity.UploadAvatar(r.Context(), userID, file, part.Header.Get("Content-Type"))
		_ = part.Close()
		if err != nil {
			if file.err != nil {
				h.avatarReadError(r, w, file.err)
				return
			}
			handleError(r.Context(), h.logger, w, "upload avatar", err)
			return
		}

		response.WriteJSON(w, http.StatusOK, dto.AvatarResponse{AvatarURL: avatarURL})
		return
	}

	logError(r.Context(), h.logger, "upload avatar", errors.New("avatar file is required"))
	response.ErrorBadRequest(w)
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
	userID, ok := userIDFromContext(r)
	if !ok {
		response.ErrorUnauthorized(w)
		return
	}

	if err := h.identity.DeleteAvatar(r.Context(), userID); err != nil {
		handleError(r.Context(), h.logger, w, "delete avatar", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) avatarReadError(r *http.Request, w http.ResponseWriter, err error) {
	logError(r.Context(), h.logger, "upload avatar", err)

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		response.ErrorPayloadTooLarge(w)
		return
	}

	response.ErrorBadRequest(w)
}

func userIDFromContext(r *http.Request) (uuid.UUID, bool) {
	user, ok := middleware.UserInfoFromContext(r.Context())
	if !ok || user == nil || user.UserID == uuid.Nil {
		return uuid.Nil, false
	}

	return user.UserID, true
}

type readErrorRecorder struct {
	reader io.Reader
	err    error
}

func (r *readErrorRecorder) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		r.err = err
	}

	return n, err
}
