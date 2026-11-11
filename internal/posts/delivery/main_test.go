package posthandlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	posthandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/delivery/mock_posts_delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func newRouter(t *testing.T) (*mux.Router, *mock_posts_delivery.MockPostUsecase) {
	t.Helper()

	uc := mock_posts_delivery.NewMockPostUsecase(t)
	h := posthandlers.NewHandler(uc, testutils.SetupValidator(), testutils.SetupLogger())
	router := mux.NewRouter()
	h.RegisterRoutes(router, router)
	return router, uc
}

func newRequest(method, target, body string, userID *uuid.UUID) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if userID != nil {
		r = testutils.WithUser(r, *userID)
	}
	return r
}

func serve(router http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func wrapped(err error) error {
	return fmt.Errorf("usecase: %w", err)
}

func previews(n int) []*dto.PostPreview {
	items := make([]*dto.PostPreview, n)
	for i := range items {
		publishedAt := postTime.Add(-time.Duration(i) * time.Hour)
		items[i] = &dto.PostPreview{
			ID:                   uuid.New(),
			Author:               dto.AuthorResponse{ID: postAuthorID, Username: authorUsername, Nickname: "Иван"},
			Title:                "Пост",
			BodyPreview:          "Текст поста",
			PublishedOrUpdatedAt: &publishedAt,
			LikesCount:           int64(100 - i),
		}
	}
	return items
}

type cursorKind int

const (
	timedKind cursorKind = iota
	popularKind
)

func nextCursor(kind cursorKind, last *dto.PostPreview) any {
	if kind == timedKind {
		return &dto.TimedPostsCursor{PublishedOrUpdatedAt: *last.PublishedOrUpdatedAt, PostID: last.ID}
	}
	return &dto.PopularPostsCursor{Likes: last.LikesCount, PublishedAt: *last.PublishedOrUpdatedAt, PostID: last.ID}
}

func encodeCursor(t *testing.T, cur any) string {
	t.Helper()

	var (
		raw *string
		err error
	)
	switch c := cur.(type) {
	case *dto.TimedPostsCursor:
		raw, err = posthandlers.TimedPostsCursor.Encode(c)
	case *dto.PopularPostsCursor:
		raw, err = posthandlers.PopularPostsCursor.Encode(c)
	}
	require.NoError(t, err)
	require.NotNil(t, raw)
	return *raw
}

func validCursor(t *testing.T, kind cursorKind) string {
	return encodeCursor(t, nextCursor(kind, previews(1)[0]))
}

func otherKindCursor(t *testing.T, kind cursorKind) string {
	if kind == timedKind {
		return validCursor(t, popularKind)
	}
	return validCursor(t, timedKind)
}

func pageOf(kind cursorKind, items []*dto.PostPreview, next any) any {
	if kind == timedKind {
		p := &dto.TimedPostsPage{Items: items}
		if next != nil {
			p.Next = next.(*dto.TimedPostsCursor)
		}
		return p
	}
	p := &dto.PopularPostsPage{Items: items}
	if next != nil {
		p.Next = next.(*dto.PopularPostsCursor)
	}
	return p
}

func timed(raw string) *dto.TimedPostsCursor {
	c, _ := posthandlers.TimedPostsCursor.Decode(raw)
	return c
}

func popular(raw string) *dto.PopularPostsCursor {
	c, _ := posthandlers.PopularPostsCursor.Decode(raw)
	return c
}
