package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFeedFilters_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		target string
		code   string
	}{
		{name: "unknown tag", target: "/feed?sort=new&tag=Неизвестно", code: "bad request"},
		{name: "repeated tag", target: "/feed?sort=new&tag=Игры&tag=Другое", code: "bad request"},
		{name: "unknown attachment", target: "/feed?sort=new&attachment_type=text", code: "bad request"},
		{name: "repeated attachment", target: "/feed?sort=new&attachment_type=video&attachment_type=image", code: "bad request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, _ := newRouter(t)

			recorder := serve(router, httptest.NewRequest(http.MethodGet, tt.target, nil))

			require.Equal(t, http.StatusBadRequest, recorder.Code)

			var body map[string]string
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, tt.code, body["error"])
		})
	}
}

func TestFeedFilters_PassedToUsecase(t *testing.T) {
	router, uc := newRouter(t)

	uc.EXPECT().GetNewPosts(
		mock.Anything,
		(*uuid.UUID)(nil),
		(*dto.TimedPostsCursor)(nil),
		20,
		dto.FeedFilters{Query: "выпуск", Tag: "Игры", AttachmentType: "video"},
	).Return(&dto.TimedPostsPage{}, nil)

	recorder := serve(router, httptest.NewRequest(
		http.MethodGet,
		"/feed?sort=new&q=%20выпуск%20&tag=Игры&attachment_type=video",
		nil,
	))

	require.Equal(t, http.StatusOK, recorder.Code)
}
