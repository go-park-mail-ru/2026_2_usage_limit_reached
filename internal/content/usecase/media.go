package usecase

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/abema/go-mp4"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/models"
	"github.com/google/uuid"

	_ "golang.org/x/image/webp"
	_ "image/jpeg"
	_ "image/png"
)

var (
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrMediaTooLarge        = errors.New("media too large")
	ErrRangeNotSatisfiable  = errors.New("range not satisfiable")
)

const (
	mediaKeyPrefix      = "posts"
	mediaDeleteTimeout  = 5 * time.Second
	maxMediaFilenameLen = 255
	mediaSniffSize      = 512
	attachmentURLPrefix = "/attachments"
)

// MediaStorage — объектное хранилище медиа; удовлетворяется pkg/s3.Client
type MediaStorage interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type mediaFormat struct {
	attachmentType dto.AttachmentType
	contentType    string
	ext            string
}

var imageFormats = map[string]mediaFormat{
	"jpeg": {dto.AttachmentTypeImage, "image/jpeg", ".jpg"},
	"png":  {dto.AttachmentTypeImage, "image/png", ".png"},
	"webp": {dto.AttachmentTypeImage, "image/webp", ".webp"},
}

// contentFormats — сниффинг не-картинок по http.DetectContentType.
// ZIP-сигнатура покрывает DOCX/PPTX/XLSX и обычные архивы одинаково.
var contentFormats = map[string]mediaFormat{
	"video/mp4":       {dto.AttachmentTypeVideo, "video/mp4", ".mp4"},
	"audio/mpeg":      {dto.AttachmentTypeAudio, "audio/mpeg", ".mp3"},
	"application/pdf": {dto.AttachmentTypeFile, "application/pdf", ".pdf"},
	"application/zip": {dto.AttachmentTypeFile, "application/zip", ".zip"},
}

// allowedMp4Codecs — форматы sample entry, которые надёжно играют в браузерах:
// видеодорожки H.264 (avc1/avc3) и аудиодорожки AAC (mp4a)
var allowedMp4Codecs = map[string]bool{
	"avc1": true,
	"avc3": true,
	"mp4a": true,
}

// AttachmentRange — разобранный заголовок Range: Start/End в терминах
// "bytes=start-end"; End == -1 — до конца; Start < 0 — суффикс "bytes=-n".
type AttachmentRange struct {
	Start int64
	End   int64
}

// ResolveRange переводит AttachmentRange в (offset, length) для MediaStorage.
// length < 0 — до конца файла. Некорректный диапазон → offset = -1.
func ResolveRange(rng *AttachmentRange, size int64) (offset, length int64) {
	if rng == nil {
		return 0, -1
	}

	if rng.Start < 0 { // "bytes=-n" — последние n байт
		n := -rng.Start
		if n > size {
			n = size
		}
		return size - n, -1
	}

	if rng.Start >= size {
		return -1, 0
	}

	if rng.End < 0 || rng.End >= size {
		return rng.Start, -1
	}

	if rng.End < rng.Start {
		return -1, 0
	}

	return rng.Start, rng.End - rng.Start + 1
}

// UploadAttachment сохраняет файл в S3 и реестре media и возвращает данные для маркера
func (uc *ContentUsecase) UploadAttachment(ctx context.Context, userID *uuid.UUID, file io.ReadSeeker, size int64, filename string) (*dto.Attachment, error) {
	if userID == nil {
		return nil, fmt.Errorf("upload attachment: %w", ErrUnauthorized)
	}
	if size <= 0 {
		return nil, fmt.Errorf("upload attachment: empty file: %w", ErrUnsupportedMediaType)
	}

	filename, err := cleanFilename(filename)
	if err != nil {
		return nil, fmt.Errorf("upload attachment: %w", err)
	}

	pending, err := uc.repo.CountPendingMedia(ctx, *userID)
	if err != nil {
		return nil, fmt.Errorf("upload attachment: %w", err)
	}
	if int(pending) >= uc.attLimits.MaxPendingMedia() {
		return nil, fmt.Errorf("upload attachment: %w: too many pending files", ErrValidation)
	}

	format, err := sniffMedia(file)
	if err != nil {
		return nil, fmt.Errorf("upload attachment: %w", err)
	}

	if format.attachmentType == dto.AttachmentTypeVideo {
		if err := validateMp4Codecs(file); err != nil {
			return nil, fmt.Errorf("upload attachment: %w", err)
		}
	}

	if size > uc.maxAttachmentSize(format.attachmentType) {
		return nil, fmt.Errorf("upload attachment: %w", ErrMediaTooLarge)
	}

	key := path.Join(mediaKeyPrefix, userID.String(), uuid.NewString()+format.ext)
	if err := uc.media.Put(ctx, key, file, size, format.contentType); err != nil {
		return nil, fmt.Errorf("upload attachment: put media: %w", err)
	}

	created, err := uc.repo.CreateMedia(ctx, models.Media{
		AuthorID:  *userID,
		MediaType: string(format.attachmentType),
		MimeType:  format.contentType,
		MediaKey:  key,
		Filename:  filename,
		SizeBytes: size,
	})
	if err != nil {
		if delErr := uc.deleteMediaObject(ctx, key); delErr != nil {
			return nil, errors.Join(fmt.Errorf("upload attachment: %w", mapRepoError("create media", err)), delErr)
		}
		return nil, mapRepoError("upload attachment", err)
	}

	return &dto.Attachment{
		ID:        created.ID,
		MediaType: format.attachmentType,
		MimeType:  format.contentType,
		Name:      filename,
		SizeBytes: size,
		URL:       attachmentURLPrefix + "/" + created.ID.String(),
	}, nil
}

// OpenAttachment проверяет доступ, открывает поток и возвращает метаданные.
// rng == nil — файл целиком.
func (uc *ContentUsecase) OpenAttachment(ctx context.Context, userID *uuid.UUID, id uuid.UUID, rng *AttachmentRange) (*dto.Attachment, io.ReadCloser, error) {
	stored, err := uc.repo.GetMediaForAccess(ctx, userID, id)
	if err != nil {
		return nil, nil, mapRepoError("open attachment", err)
	}

	offset, length := ResolveRange(rng, stored.SizeBytes)
	if offset < 0 {
		return nil, nil, fmt.Errorf("open attachment: %w", ErrRangeNotSatisfiable)
	}

	body, err := uc.media.Open(ctx, stored.MediaKey, offset, length)
	if err != nil {
		return nil, nil, fmt.Errorf("open attachment: %w", err)
	}

	return &dto.Attachment{
		ID:        stored.ID,
		MediaType: dto.AttachmentType(stored.MediaType),
		MimeType:  stored.MimeType,
		Name:      stored.Filename,
		SizeBytes: stored.SizeBytes,
	}, body, nil
}

func (uc *ContentUsecase) maxAttachmentSize(t dto.AttachmentType) int64 {
	switch t {
	case dto.AttachmentTypeImage:
		return uc.attLimits.MaxImageSize()
	case dto.AttachmentTypeVideo:
		return uc.attLimits.MaxVideoSize()
	case dto.AttachmentTypeAudio:
		return uc.attLimits.MaxAudioSize()
	default:
		return uc.attLimits.MaxFileSize()
	}
}

// sniffMedia определяет формат по содержимому файла, а не по заявленным клиентом данным
func sniffMedia(file io.ReadSeeker) (mediaFormat, error) {
	if _, name, err := image.DecodeConfig(file); err == nil {
		if format, ok := imageFormats[name]; ok {
			return format, nil
		}
		return mediaFormat{}, fmt.Errorf("sniff media: %w", ErrUnsupportedMediaType)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return mediaFormat{}, fmt.Errorf("sniff media: rewind: %w", err)
	}

	buf := make([]byte, mediaSniffSize)
	n, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return mediaFormat{}, fmt.Errorf("sniff media: read header: %w", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return mediaFormat{}, fmt.Errorf("sniff media: rewind: %w", err)
	}

	if format, ok := contentFormats[http.DetectContentType(buf[:n])]; ok {
		return format, nil
	}

	return mediaFormat{}, fmt.Errorf("sniff media: %w", ErrUnsupportedMediaType)
}

// validateMp4Codecs проверяет, что все sample entry в stsd — H.264 (avc1/avc3)
// или AAC (mp4a). Неразобранная структура и прочие форматы (hvc1, av01 и т.п.)
// отклоняются: в браузерах надёжно играет только H.264 + AAC.
func validateMp4Codecs(file io.ReadSeeker) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("validate mp4: rewind: %w", err)
	}

	var (
		codecs []string
		buf    bytes.Buffer
	)

	if _, err := mp4.ReadBoxStructure(file, func(h *mp4.ReadHandle) (interface{}, error) {
		if h.BoxInfo.Type != mp4.BoxTypeStsd() {
			return nil, nil
		}

		buf.Reset()
		if _, err := h.ReadData(&buf); err != nil {
			return nil, err
		}

		codecs = append(codecs, parseStsdEntries(buf.Bytes())...)

		return nil, nil
	}); err != nil {
		return fmt.Errorf("validate mp4: %w: %w", ErrUnsupportedMediaType, err)
	}

	if len(codecs) == 0 {
		return fmt.Errorf("validate mp4: %w: no sample entries", ErrUnsupportedMediaType)
	}

	for _, codec := range codecs {
		if !allowedMp4Codecs[codec] {
			return fmt.Errorf("validate mp4: %w: codec %q", ErrUnsupportedMediaType, codec)
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("validate mp4: rewind: %w", err)
	}

	return nil
}

// parseStsdEntries возвращает форматы (4CC) sample entry из payload бокса stsd:
// 4 байта version/flags + 4 байта entry_count, далее записи size(4) + type(4)
func parseStsdEntries(payload []byte) []string {
	const (
		headerLen = 8
		entryHead = 8
	)

	if len(payload) < headerLen {
		return nil
	}

	entries := make([]string, 0, 2)
	for offset := headerLen; offset+entryHead <= len(payload); {
		size := int(binary.BigEndian.Uint32(payload[offset : offset+4]))
		if size < entryHead || offset+size > len(payload) {
			break
		}

		entries = append(entries, string(payload[offset+4:offset+entryHead]))
		offset += size
	}

	return entries
}

// cleanFilename приводит клиентское имя к безопасной строке для отображения:
// без каталогов, без пробелов по краям, не длиннее maxMediaFilenameLen рун.
func cleanFilename(raw string) (string, error) {
	filename := strings.TrimSpace(path.Base(strings.ReplaceAll(raw, "\\", "/")))
	if filename == "" || filename == "." || filename == "/" {
		return "", fmt.Errorf("filename is empty: %w", ErrValidation)
	}

	if runes := []rune(filename); len(runes) > maxMediaFilenameLen {
		filename = string(runes[:maxMediaFilenameLen])
	}

	return filename, nil
}

func (uc *ContentUsecase) deleteMediaObject(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaDeleteTimeout)
	defer cancel()

	return uc.media.Delete(ctx, key)
}

// deleteDetachedMedia вычищает объекты S3, потерявшие последнюю связку.
// Ошибки логируются и не влияют на результат операции.
func (uc *ContentUsecase) deleteDetachedMedia(ctx context.Context, keys []string) {
	for _, key := range keys {
		if err := uc.deleteMediaObject(ctx, key); err != nil && uc.logger != nil {
			uc.logger.ErrorContext(ctx, "delete detached media error",
				slog.String("key", key),
				slog.String("error", err.Error()),
			)
		}
	}
}
