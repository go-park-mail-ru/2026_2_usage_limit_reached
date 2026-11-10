package posthandlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/validator"
	posthandlers "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/delivery/mock_posts_delivery"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	postusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/response"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	defaultLimit   = 20
	authorUsername = "ivan001"
)

var (
	postUserID   = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	postAuthorID = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	postID       = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	postTime     = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	anonymous    = (*uuid.UUID)(nil)
	technicalErr = errors.New("post storage unavailable")

	postRequest  = dto.PostRequest{Title: "Мой первый пост", Body: "Текст поста", Status: "published"}
	editRequest  = dto.PostRequest{Title: "Мой первый пост", Body: "Текст поста", Status: "published", LastUpdatedAt: &postTime}
	postJSON     = `{"title":"Мой первый пост","body":"Текст поста","status":"published"}`
	editPostJSON = `{"title":"Мой первый пост","body":"Текст поста","status":"published","last_updated_at":"2026-09-01T12:00:00Z"}`
)

type listEndpoint struct {
	name   string
	target string
	kind   cursorKind
	expect func(uc *mock_posts_delivery.MockPostUsecase, userID *uuid.UUID, rawCursor string, limit int) *mock.Call
}

func listEndpoints() []listEndpoint {
	return []listEndpoint{
		{
			name: "feed new", target: "/feed?sort=new", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetNewPosts(mock.Anything, u, timed(raw), limit).Call
			},
		},
		{
			name: "feed old", target: "/feed?sort=old", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetOldPosts(mock.Anything, u, timed(raw), limit).Call
			},
		},
		{
			name: "feed popular", target: "/feed?sort=popular", kind: popularKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetPopularPosts(mock.Anything, u, popular(raw), limit).Call
			},
		},
		{
			name: "user new", target: "/users/" + authorUsername + "/posts?sort=new", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetUserNewPosts(mock.Anything, u, authorUsername, timed(raw), limit).Call
			},
		},
		{
			name: "user old", target: "/users/" + authorUsername + "/posts?sort=old", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetUserOldPosts(mock.Anything, u, authorUsername, timed(raw), limit).Call
			},
		},
		{
			name: "user popular", target: "/users/" + authorUsername + "/posts?sort=popular", kind: popularKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetUserPopularPosts(mock.Anything, u, authorUsername, popular(raw), limit).Call
			},
		},
		{
			name: "my new posts", target: "/profile/me/posts?sort=new", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetMyNewPosts(mock.Anything, u, timed(raw), limit).Call
			},
		},
		{
			name: "my old posts", target: "/profile/me/posts?sort=old", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetMyOldPosts(mock.Anything, u, timed(raw), limit).Call
			},
		},
		{
			name: "my popular posts", target: "/profile/me/posts?sort=popular", kind: popularKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetMyPopularPosts(mock.Anything, u, popular(raw), limit).Call
			},
		},
		{
			name: "my new drafts", target: "/profile/me/drafts?sort=new", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetMyNewDrafts(mock.Anything, u, timed(raw), limit).Call
			},
		},
		{
			name: "my old drafts", target: "/profile/me/drafts?sort=old", kind: timedKind,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID, raw string, limit int) *mock.Call {
				return uc.EXPECT().GetMyOldDrafts(mock.Anything, u, timed(raw), limit).Call
			},
		},
	}
}

func TestPostHandler_List_Pages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		withNext bool
	}{
		{name: "page with next cursor", withNext: true},
		{name: "last page"},
	}
	for _, e := range listEndpoints() {
		for _, tt := range tests {
			t.Run(e.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				items := previews(2)
				var next any
				if tt.withNext {
					next = nextCursor(e.kind, items[1])
				}
				router, uc := newRouter(t)
				e.expect(uc, &postUserID, "", defaultLimit).Return(pageOf(e.kind, items, next), nil)

				rec := serve(router, newRequest(http.MethodGet, e.target, "", &postUserID))

				require.Equal(t, http.StatusOK, rec.Code)
				got := testutils.DecodeJSON[dto.PostsPageResponse](t, rec)
				require.Equal(t, items, got.Items)
				if tt.withNext {
					require.NotNil(t, got.NextCursor)
					require.Equal(t, encodeCursor(t, next), *got.NextCursor)
				} else {
					require.Nil(t, got.NextCursor)
				}
			})
		}
	}
}

func TestPostHandler_List_CursorAndLimit(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()

			cursor := validCursor(t, e.kind)
			router, uc := newRouter(t)
			e.expect(uc, &postUserID, cursor, 5).Return(pageOf(e.kind, previews(1), nil), nil)

			rec := serve(router, newRequest(http.MethodGet, e.target+"&limit=5&cursor="+cursor, "", &postUserID))

			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestPostHandler_List_LimitOutOfRange(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		for _, limit := range []string{"0", "-1", "51", "много"} {
			t.Run(e.name+"/"+limit, func(t *testing.T) {
				t.Parallel()

				router, uc := newRouter(t)
				e.expect(uc, &postUserID, "", defaultLimit).Return(pageOf(e.kind, previews(0), nil), nil)

				rec := serve(router, newRequest(http.MethodGet, e.target+"&limit="+limit, "", &postUserID))

				require.Equal(t, http.StatusOK, rec.Code)
			})
		}
	}
}

func TestPostHandler_List_Anonymous(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			e.expect(uc, anonymous, "", defaultLimit).Return(pageOf(e.kind, previews(1), nil), nil)

			rec := serve(router, newRequest(http.MethodGet, e.target, "", anonymous))

			require.Equal(t, http.StatusOK, rec.Code)
		})
	}
}

func TestPostHandler_List_InvalidCursor(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		cursors := map[string]string{
			"not base64":       "not*base64!",
			"not json":         "bm90LWpzb24",
			"other sort order": otherKindCursor(t, e.kind),
		}
		for name, cursor := range cursors {
			t.Run(e.name+"/"+name, func(t *testing.T) {
				t.Parallel()

				router, _ := newRouter(t)

				rec := serve(router, newRequest(http.MethodGet, e.target+"&cursor="+cursor, "", &postUserID))

				require.Equal(t, http.StatusBadRequest, rec.Code)
			})
		}
	}
}

func TestPostHandler_List_InvalidNextCursor(t *testing.T) {
	t.Parallel()

	for _, e := range listEndpoints() {
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()

			zeroCursor := map[cursorKind]any{timedKind: &dto.TimedPostsCursor{}, popularKind: &dto.PopularPostsCursor{}}
			router, uc := newRouter(t)
			e.expect(uc, &postUserID, "", defaultLimit).Return(pageOf(e.kind, previews(1), zeroCursor[e.kind]), nil)

			rec := serve(router, newRequest(http.MethodGet, e.target, "", &postUserID))

			require.Equal(t, http.StatusInternalServerError, rec.Code)
		})
	}
}

func TestPostHandler_List_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ucErr      error
		wantStatus int
	}{
		{name: "anonymous on private list", ucErr: postusecase.ErrUnauthorized, wantStatus: http.StatusUnauthorized},
		{name: "unknown author", ucErr: postusecase.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "storage failure", ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, e := range listEndpoints() {
		for _, tt := range tests {
			t.Run(e.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				router, uc := newRouter(t)
				e.expect(uc, &postUserID, "", defaultLimit).Return(nil, wrapped(tt.ucErr))

				rec := serve(router, newRequest(http.MethodGet, e.target, "", &postUserID))

				require.Equal(t, tt.wantStatus, rec.Code)
			})
		}
	}
}

func TestPostHandler_List_InvalidSort(t *testing.T) {
	t.Parallel()

	targets := []string{
		"/feed",
		"/feed?sort=top",
		"/users/" + authorUsername + "/posts",
		"/users/" + authorUsername + "/posts?sort=top",
		"/profile/me/posts",
		"/profile/me/posts?sort=top",
		"/profile/me/drafts?sort=popular",
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			router, _ := newRouter(t)

			rec := serve(router, newRequest(http.MethodGet, target, "", &postUserID))

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestPostHandler_UserPosts_EmptyUsername(t *testing.T) {
	t.Parallel()

	h := posthandlers.NewHandler(mock_posts_delivery.NewMockPostUsecase(t), testutils.SetupValidator(), testutils.SetupLogger())
	rec := httptest.NewRecorder()

	h.UserPosts(rec, httptest.NewRequest(http.MethodGet, "/users//posts?sort=new", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostHandler_ViewPost_Success(t *testing.T) {
	t.Parallel()

	post := &dto.PostInfo{
		ID:          postID,
		Author:      dto.AuthorResponse{ID: postAuthorID, Username: authorUsername, Nickname: "Иван"},
		Title:       "Мой первый пост",
		Body:        "Текст поста",
		Status:      "published",
		PublishedAt: &postTime,
		CreatedAt:   postTime,
		UpdatedAt:   postTime,
		LikesCount:  12,
		LikedByUser: true,
	}
	tests := []struct {
		name   string
		userID *uuid.UUID
	}{
		{name: "authorized viewer", userID: &postUserID},
		{name: "anonymous viewer", userID: anonymous},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			uc.EXPECT().GetPost(mock.Anything, tt.userID, postID).Return(post, nil)

			rec := serve(router, newRequest(http.MethodGet, "/posts/"+postID.String(), "", tt.userID))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, *post, testutils.DecodeJSON[dto.PostInfo](t, rec))
		})
	}
}

func TestPostHandler_ErrorStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ucErr      error
		wantStatus int
	}{
		{name: "conflict", ucErr: postusecase.ErrConflict, wantStatus: http.StatusConflict},
		{name: "forbidden", ucErr: postusecase.ErrForbidden, wantStatus: http.StatusForbidden},
		{name: "unauthorized", ucErr: postusecase.ErrUnauthorized, wantStatus: http.StatusUnauthorized},
		{name: "not found", ucErr: postusecase.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "usecase validation", ucErr: postusecase.ErrValidation, wantStatus: http.StatusBadRequest},
		{name: "request validation", ucErr: validator.ErrValidation, wantStatus: http.StatusBadRequest},
		{name: "storage failure", ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			uc.EXPECT().GetPost(mock.Anything, &postUserID, postID).Return(nil, wrapped(tt.ucErr))

			rec := serve(router, newRequest(http.MethodGet, "/posts/"+postID.String(), "", &postUserID))

			require.Equal(t, tt.wantStatus, rec.Code)
			require.NotEmpty(t, testutils.DecodeJSON[response.ErrorResponse](t, rec).Error)
		})
	}
}

func TestPostHandler_InvalidPostID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method string
		target string
		body   string
	}{
		{method: http.MethodGet, target: "/posts/not-a-uuid"},
		{method: http.MethodPut, target: "/posts/not-a-uuid", body: editPostJSON},
		{method: http.MethodDelete, target: "/posts/not-a-uuid"},
		{method: http.MethodPost, target: "/posts/not-a-uuid/like"},
		{method: http.MethodDelete, target: "/posts/not-a-uuid/like"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.target, func(t *testing.T) {
			t.Parallel()

			router, _ := newRouter(t)

			rec := serve(router, newRequest(tt.method, tt.target, tt.body, &postUserID))

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestPostHandler_NewPost_Success(t *testing.T) {
	t.Parallel()

	router, uc := newRouter(t)
	uc.EXPECT().CreatePost(mock.Anything, &postUserID, postRequest).Return(postID, nil)

	rec := serve(router, newRequest(http.MethodPost, "/posts", postJSON, &postUserID))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, dto.CreatePostResponse{ID: postID}, testutils.DecodeJSON[dto.CreatePostResponse](t, rec))
}

func invalidPostBodies() map[string]string {
	return map[string]string{
		"malformed json":  `{"title":`,
		"wrong type":      `{"title":1,"body":"Текст поста","status":"published"}`,
		"empty title":     `{"title":"","body":"Текст поста","status":"published"}`,
		"blank title":     `{"title":"   ","body":"Текст поста","status":"published"}`,
		"too long title":  `{"title":"` + strings.Repeat("я", 256) + `","body":"Текст поста","status":"published"}`,
		"empty body":      `{"title":"Мой первый пост","body":"","status":"published"}`,
		"unknown status":  `{"title":"Мой первый пост","body":"Текст поста","status":"deleted"}`,
		"missing status":  `{"title":"Мой первый пост","body":"Текст поста"}`,
		"empty json body": ``,
	}
}

func TestPostHandler_NewPost_InvalidRequest(t *testing.T) {
	t.Parallel()

	for name, body := range invalidPostBodies() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			router, _ := newRouter(t)

			rec := serve(router, newRequest(http.MethodPost, "/posts", body, &postUserID))

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestPostHandler_NewPost_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		userID     *uuid.UUID
		ucErr      error
		wantStatus int
	}{
		{name: "anonymous user", userID: anonymous, ucErr: postusecase.ErrUnauthorized, wantStatus: http.StatusUnauthorized},
		{name: "reader is not an author", userID: &postUserID, ucErr: postusecase.ErrForbidden, wantStatus: http.StatusForbidden},
		{name: "storage failure", userID: &postUserID, ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			uc.EXPECT().CreatePost(mock.Anything, tt.userID, postRequest).Return(uuid.Nil, wrapped(tt.ucErr))

			rec := serve(router, newRequest(http.MethodPost, "/posts", postJSON, tt.userID))

			require.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestPostHandler_EditPost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ucErr      error
		wantStatus int
	}{
		{name: "actual version", wantStatus: http.StatusOK},
		{name: "stale version", ucErr: postusecase.ErrConflict, wantStatus: http.StatusConflict},
		{name: "foreign or unknown post", ucErr: postusecase.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "storage failure", ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var ucErr error
			if tt.ucErr != nil {
				ucErr = wrapped(tt.ucErr)
			}
			router, uc := newRouter(t)
			uc.EXPECT().UpdatePost(mock.Anything, &postUserID, postID, editRequest, postTime).Return(ucErr)

			rec := serve(router, newRequest(http.MethodPut, "/posts/"+postID.String(), editPostJSON, &postUserID))

			require.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestPostHandler_EditPost_InvalidRequest(t *testing.T) {
	t.Parallel()

	bodies := invalidPostBodies()
	bodies["missing last_updated_at"] = postJSON
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			router, _ := newRouter(t)

			rec := serve(router, newRequest(http.MethodPut, "/posts/"+postID.String(), body, &postUserID))

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestPostHandler_DeletePost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		userID     *uuid.UUID
		ucErr      error
		wantStatus int
	}{
		{name: "own post", userID: &postUserID, wantStatus: http.StatusOK},
		{name: "anonymous user", userID: anonymous, ucErr: postusecase.ErrUnauthorized, wantStatus: http.StatusUnauthorized},
		{name: "foreign or unknown post", userID: &postUserID, ucErr: postusecase.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "storage failure", userID: &postUserID, ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var ucErr error
			if tt.ucErr != nil {
				ucErr = wrapped(tt.ucErr)
			}
			router, uc := newRouter(t)
			uc.EXPECT().DeletePost(mock.Anything, tt.userID, postID).Return(ucErr)

			rec := serve(router, newRequest(http.MethodDelete, "/posts/"+postID.String(), "", tt.userID))

			require.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

type likeEndpoint struct {
	name      string
	method    string
	likedByMe bool
	expect    func(uc *mock_posts_delivery.MockPostUsecase, userID *uuid.UUID) *mock.Call
}

func likeEndpoints() []likeEndpoint {
	return []likeEndpoint{
		{
			name: "like", method: http.MethodPost, likedByMe: true,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID) *mock.Call {
				return uc.EXPECT().LikePost(mock.Anything, u, postID).Call
			},
		},
		{
			name: "unlike", method: http.MethodDelete,
			expect: func(uc *mock_posts_delivery.MockPostUsecase, u *uuid.UUID) *mock.Call {
				return uc.EXPECT().DeletePostLike(mock.Anything, u, postID).Call
			},
		},
	}
}

func TestPostHandler_Likes_Success(t *testing.T) {
	t.Parallel()

	for _, e := range likeEndpoints() {
		t.Run(e.name, func(t *testing.T) {
			t.Parallel()

			router, uc := newRouter(t)
			e.expect(uc, &postUserID).Return(int64(13), nil)

			rec := serve(router, newRequest(e.method, "/posts/"+postID.String()+"/like", "", &postUserID))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t,
				dto.PostLikeResponse{LikesCount: 13, LikedByUser: e.likedByMe},
				testutils.DecodeJSON[dto.PostLikeResponse](t, rec),
			)
		})
	}
}

func TestPostHandler_Likes_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		userID     *uuid.UUID
		ucErr      error
		wantStatus int
	}{
		{name: "anonymous user", userID: anonymous, ucErr: postusecase.ErrUnauthorized, wantStatus: http.StatusUnauthorized},
		{name: "unavailable post", userID: &postUserID, ucErr: postusecase.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "storage failure", userID: &postUserID, ucErr: technicalErr, wantStatus: http.StatusInternalServerError},
	}
	for _, e := range likeEndpoints() {
		for _, tt := range tests {
			t.Run(e.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				router, uc := newRouter(t)
				e.expect(uc, tt.userID).Return(int64(0), wrapped(tt.ucErr))

				rec := serve(router, newRequest(e.method, "/posts/"+postID.String()+"/like", "", tt.userID))

				require.Equal(t, tt.wantStatus, rec.Code)
			})
		}
	}
}
