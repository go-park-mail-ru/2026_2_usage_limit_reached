package handlers_test

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	contentusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var (
	attachmentID  = uuid.MustParse("00000000-0000-4000-8000-000000000301")
	attachmentURL = "/attachments/" + attachmentID.String()
)

func multipartFile(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	return body, writer.FormDataContentType()
}

func uploadRequest(t *testing.T, body io.Reader, contentType string) *http.Request {
	t.Helper()

	r := httptest.NewRequest(http.MethodPost, "/attachments", body)
	r.Header.Set("Content-Type", contentType)

	return testutils.WithUser(r, postUserID)
}

func TestUploadAttachment(t *testing.T) {
	t.Parallel()

	router, uc := newRouter(t)
	content := []byte("image-bytes")

	uc.EXPECT().
		UploadAttachment(mock.Anything, mock.Anything, mock.Anything, int64(len(content)), "cat.png").
		Return(&dto.Attachment{
			ID:        attachmentID,
			MediaType: dto.AttachmentTypeImage,
			MimeType:  "image/png",
			Name:      "cat.png",
			SizeBytes: int64(len(content)),
			URL:       attachmentURL,
		}, nil).
		Once()

	body, contentType := multipartFile(t, "file", "cat.png", content)
	rec := serve(router, uploadRequest(t, body, contentType))

	require.Equal(t, http.StatusCreated, rec.Code)

	got := testutils.DecodeJSON[dto.Attachment](t, rec)
	require.Equal(t, attachmentURL, got.URL)
	require.Equal(t, "cat.png", got.Name)
}

func TestUploadAttachmentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "too large", err: contentusecase.ErrMediaTooLarge, code: http.StatusRequestEntityTooLarge},
		{name: "unsupported type", err: contentusecase.ErrUnsupportedMediaType, code: http.StatusUnsupportedMediaType},
		{name: "validation", err: contentusecase.ErrValidation, code: http.StatusBadRequest},
		{name: "unauthorized", err: contentusecase.ErrUnauthorized, code: http.StatusUnauthorized},
		{name: "forbidden", err: contentusecase.ErrForbidden, code: http.StatusForbidden},
		{name: "internal", err: errors.New("storage is down"), code: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			uc.EXPECT().
				UploadAttachment(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, wrapped(tt.err)).
				Once()

			body, contentType := multipartFile(t, "file", "cat.png", []byte("data"))
			rec := serve(router, uploadRequest(t, body, contentType))

			require.Equal(t, tt.code, rec.Code)
		})
	}
}

func TestUploadAttachmentAnonymous(t *testing.T) {
	t.Parallel()

	router, _ := newRouter(t)
	body, contentType := multipartFile(t, "file", "cat.png", []byte("data"))
	r := httptest.NewRequest(http.MethodPost, "/attachments", body)
	r.Header.Set("Content-Type", contentType)

	rec := serve(router, r)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUploadAttachmentWithoutFile(t *testing.T) {
	t.Parallel()

	router, _ := newRouter(t)
	body, contentType := multipartFile(t, "not-file", "cat.png", []byte("data"))

	rec := serve(router, uploadRequest(t, body, contentType))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUploadAttachmentNotMultipart(t *testing.T) {
	t.Parallel()

	router, _ := newRouter(t)
	r := uploadRequest(t, strings.NewReader("plain"), "application/json")

	rec := serve(router, r)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetAttachment(t *testing.T) {
	t.Parallel()

	attachment := &dto.Attachment{
		ID:        attachmentID,
		MediaType: dto.AttachmentTypeImage,
		MimeType:  "image/png",
		Name:      "cat.png",
		SizeBytes: 100,
	}

	tests := []struct {
		name            string
		method          string
		rangeHeader     string
		wantCode        int
		wantRange       string
		wantLength      string
		wantDisposition string
		wantBody        bool
	}{
		{
			name:            "full file",
			method:          http.MethodGet,
			wantCode:        http.StatusOK,
			wantLength:      "100",
			wantDisposition: "inline",
			wantBody:        true,
		},
		{
			name:            "bounded range",
			method:          http.MethodGet,
			rangeHeader:     "bytes=2-5",
			wantCode:        http.StatusPartialContent,
			wantRange:       "bytes 2-5/100",
			wantLength:      "4",
			wantDisposition: "inline",
			wantBody:        true,
		},
		{
			name:            "suffix range",
			method:          http.MethodGet,
			rangeHeader:     "bytes=-3",
			wantCode:        http.StatusPartialContent,
			wantRange:       "bytes 97-99/100",
			wantLength:      "3",
			wantDisposition: "inline",
			wantBody:        true,
		},
		{
			name:            "multi range falls back to full file",
			method:          http.MethodGet,
			rangeHeader:     "bytes=0-1,4-5",
			wantCode:        http.StatusOK,
			wantLength:      "100",
			wantDisposition: "inline",
			wantBody:        true,
		},
		{
			name:            "head returns headers only",
			method:          http.MethodHead,
			wantCode:        http.StatusOK,
			wantLength:      "100",
			wantDisposition: "inline",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			uc.EXPECT().
				OpenAttachment(mock.Anything, mock.Anything, attachmentID, mock.Anything).
				Return(attachment, io.NopCloser(strings.NewReader(strings.Repeat("a", 100))), nil).
				Once()

			r := httptest.NewRequest(tt.method, "/attachments/"+attachmentID.String(), nil)
			if tt.rangeHeader != "" {
				r.Header.Set("Range", tt.rangeHeader)
			}

			rec := serve(router, r)

			require.Equal(t, tt.wantCode, rec.Code)
			require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
			require.Equal(t, "bytes", rec.Header().Get("Accept-Ranges"))
			require.Equal(t, tt.wantLength, rec.Header().Get("Content-Length"))
			require.Contains(t, rec.Header().Get("Content-Disposition"), tt.wantDisposition)
			if tt.wantRange != "" {
				require.Equal(t, tt.wantRange, rec.Header().Get("Content-Range"))
			}

			if tt.wantBody {
				require.NotZero(t, rec.Body.Len())
			} else {
				require.Zero(t, rec.Body.Len())
			}
		})
	}
}

func TestGetAttachmentFileDisposition(t *testing.T) {
	t.Parallel()

	router, uc := newRouter(t)
	uc.EXPECT().
		OpenAttachment(mock.Anything, mock.Anything, attachmentID, mock.Anything).
		Return(&dto.Attachment{
			ID:        attachmentID,
			MediaType: dto.AttachmentTypeFile,
			MimeType:  "application/pdf",
			Name:      "report.pdf",
			SizeBytes: 10,
		}, io.NopCloser(strings.NewReader(strings.Repeat("a", 10))), nil).
		Once()

	rec := serve(router, httptest.NewRequest(http.MethodGet, "/attachments/"+attachmentID.String(), nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Disposition"), "attachment;")
	require.Contains(t, rec.Header().Get("Content-Disposition"), "report.pdf")
}

func TestGetAttachmentErrors(t *testing.T) {
	t.Parallel()

	t.Run("range not satisfiable", func(t *testing.T) {
		t.Parallel()

		router, uc := newRouter(t)
		uc.EXPECT().
			OpenAttachment(mock.Anything, mock.Anything, attachmentID, mock.Anything).
			Return(nil, nil, wrapped(contentusecase.ErrRangeNotSatisfiable)).
			Once()

		r := httptest.NewRequest(http.MethodGet, "/attachments/"+attachmentID.String(), nil)
		r.Header.Set("Range", "bytes=500-")

		rec := serve(router, r)

		require.Equal(t, http.StatusRequestedRangeNotSatisfiable, rec.Code)
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()

		router, uc := newRouter(t)
		uc.EXPECT().
			OpenAttachment(mock.Anything, mock.Anything, attachmentID, mock.Anything).
			Return(nil, nil, wrapped(contentusecase.ErrNotFound)).
			Once()

		rec := serve(router, httptest.NewRequest(http.MethodGet, "/attachments/"+attachmentID.String(), nil))

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()

		router, _ := newRouter(t)

		rec := serve(router, httptest.NewRequest(http.MethodGet, "/attachments/not-a-uuid", nil))

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
