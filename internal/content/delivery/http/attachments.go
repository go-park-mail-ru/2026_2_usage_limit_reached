package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	contentusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/middleware"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

const attachmentFormField = "file"

type AttachmentsHandler struct {
	uc        ContentUsecase
	validator *validator.Validator
	logger    *slog.Logger
}

func NewAttachmentsHandler(uc ContentUsecase, v *validator.Validator, logger *slog.Logger) *AttachmentsHandler {
	return &AttachmentsHandler{uc: uc, validator: v, logger: logger}
}

// UploadAttachment загружает вложение поста
// @Summary Загрузка вложения поста
// @Description Принимает файл в поле file multipart/form-data. Файл сохраняется в S3 и реестре media; ответ содержит id для маркера <figure data-attachment-id="...">. Тип определяется по содержимому: изображения jpeg/png/webp до 25 МиБ; видео mp4 (кодеки H.264, аудио AAC при наличии) до 1 ГиБ; аудио mp3 до 100 МиБ; файлы pdf и zip-семейство (docx, pptx, xlsx, zip) до 100 МиБ. Загружать могут только авторы.
// @Tags posts
// @Accept mpfd
// @Produce json
// @Security CookieAuth
// @Param X-CSRF-Token header string true "CSRF-токен из GET /csrf"
// @Param file formData file true "Файл вложения"
// @Success 201 {object} dto.Attachment "Вложение загружено"
// @Failure 400 {object} response.ErrorResponse "Запрос не multipart/form-data, нет поля file, пустое имя файла, или превышен лимит непривязанных файлов"
// @Failure 401 {object} response.ErrorResponse "Пользователь неавторизован"
// @Failure 403 {object} response.ErrorResponse "Пользователь не является автором, либо неверный CSRF-токен или его отсутствие"
// @Failure 413 {object} response.ErrorResponse "Файл больше лимита для своего типа"
// @Failure 415 {object} response.ErrorResponse "Неподдерживаемый формат или кодек"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /attachments [post]
func (h *AttachmentsHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := userIDFromContext(ctx)
	if userID == nil {
		response.ErrorUnauthorized(w)
		return
	}

	hardCap := h.validator.MaxVideoSize() + 1<<20
	r.Body = http.MaxBytesReader(w, r.Body, hardCap)

	reader, err := r.MultipartReader()
	if err != nil {
		logError(ctx, h.logger, "upload attachment", err)
		response.ErrorBadRequest(w)
		return
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			h.readError(ctx, w, "upload attachment", err)
			return
		}

		if part.FormName() != attachmentFormField {
			_ = part.Close()
			continue
		}

		filename := part.FileName()

		tmp, err := os.CreateTemp("", "attachment-*")
		if err != nil {
			_ = part.Close()
			h.readError(ctx, w, "upload attachment", err)
			return
		}
		defer func() {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}()

		written, err := io.Copy(tmp, io.LimitReader(part, hardCap+1))
		_ = part.Close()
		if err != nil {
			h.readError(ctx, w, "upload attachment", err)
			return
		}
		if written > hardCap {
			response.ErrorPayloadTooLarge(w)
			return
		}

		if _, err := tmp.Seek(0, io.SeekStart); err != nil {
			logError(ctx, h.logger, "upload attachment", err)
			response.ErrorInternal(w)
			return
		}

		attachment, err := h.uc.UploadAttachment(ctx, userID, tmp, written, filename)
		if err != nil {
			handleError(ctx, h.logger, w, "upload attachment", err)
			return
		}

		response.WriteJSON(w, http.StatusCreated, attachment)
		return
	}

	logError(ctx, h.logger, "upload attachment", errors.New("attachment file is required"))
	response.ErrorBadRequest(w)
}

// GetAttachment выдаёт файл вложения
// @Summary Выдача вложения
// @Description Отдаёт файл вложения. Файл доступен автору, а также всем, кому доступен опубликованный пост с этим вложением. Для закрытых постов и непривязанных файлов остальным возвращается 404. Поддерживаются Range (206) и HEAD.
// @Tags posts
// @Produce octet-stream
// @Param id path string true "ID вложения" Format(uuid)
// @Param Range header string false "Диапазон байт"
// @Success 200 {file} binary "Файл"
// @Success 206 {file} binary "Часть файла"
// @Failure 400 {object} response.ErrorResponse "Некорректный ID вложения"
// @Failure 404 {object} response.ErrorResponse "Вложение не найдено или недоступно"
// @Failure 416 {object} response.ErrorResponse "Диапазон вне файла"
// @Failure 500 {object} response.ErrorResponse "Внутренняя ошибка сервера"
// @Router /attachments/{id} [get]
func (h *AttachmentsHandler) GetAttachment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, err := uuid.Parse(mux.Vars(r)["id"])
	if err != nil {
		handleError(ctx, h.logger, w, "get attachment", fmt.Errorf("invalid attachment id: %w: %w", validator.ErrValidation, err))
		return
	}

	userID := userIDFromContext(ctx)
	rng := parseRange(r.Header.Get("Range"))

	attachment, body, err := h.uc.OpenAttachment(ctx, userID, id, rng)
	if err != nil {
		handleError(ctx, h.logger, w, "get attachment", err)
		return
	}
	defer func() {
		_ = body.Close()
	}()

	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Type", attachment.MimeType)

	if attachment.MediaType == dto.AttachmentTypeFile {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment.Name}))
	} else {
		w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", nil))
	}

	if rng == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(attachment.SizeBytes, 10))
		w.WriteHeader(http.StatusOK)
	} else {
		offset, length := contentusecase.ResolveRange(rng, attachment.SizeBytes)
		end := attachment.SizeBytes - 1
		if length > 0 {
			end = offset + length - 1
		}

		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, end, attachment.SizeBytes))
		w.Header().Set("Content-Length", strconv.FormatInt(end-offset+1, 10))
		w.WriteHeader(http.StatusPartialContent)
	}

	if r.Method == http.MethodHead {
		return
	}

	if _, err := io.Copy(w, body); err != nil {
		logError(ctx, h.logger, "get attachment", err)
	}
}

// parseRange разбирает одиночный Range. Мульти-диапазон и мусор трактуются
// как отсутствие заголовка: отдаём файл целиком.
func parseRange(raw string) *contentusecase.AttachmentRange {
	const prefix = "bytes="

	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, prefix) {
		return nil
	}

	spec := strings.TrimSpace(strings.TrimPrefix(raw, prefix))
	if spec == "" || strings.Contains(spec, ",") {
		return nil
	}

	start, end, ok := strings.Cut(spec, "-")
	if !ok {
		return nil
	}

	start, end = strings.TrimSpace(start), strings.TrimSpace(end)

	if start == "" {
		suffix, err := strconv.ParseInt(end, 10, 64)
		if err != nil || suffix <= 0 {
			return nil
		}
		return &contentusecase.AttachmentRange{Start: -suffix, End: -1}
	}

	first, err := strconv.ParseInt(start, 10, 64)
	if err != nil || first < 0 {
		return nil
	}

	if end == "" {
		return &contentusecase.AttachmentRange{Start: first, End: -1}
	}

	last, err := strconv.ParseInt(end, 10, 64)
	if err != nil || last < first {
		return nil
	}

	return &contentusecase.AttachmentRange{Start: first, End: last}
}

func (h *AttachmentsHandler) readError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	logError(ctx, h.logger, action, err)

	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		response.ErrorPayloadTooLarge(w)
		return
	}

	response.ErrorBadRequest(w)
}

func userIDFromContext(ctx context.Context) *uuid.UUID {
	user, ok := middleware.UserInfoFromContext(ctx)
	if !ok || user == nil {
		return nil
	}

	id := user.ID
	return &id
}
