package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/models"
	postusecase "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase/mock_content_usecase"
	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/pkg/testutils"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	pageLimit      = 2
	authorUsername = "ivan001"
)

var (
	postUserID    = uuid.MustParse("00000000-0000-4000-8000-000000000001")
	postAuthorID  = uuid.MustParse("00000000-0000-4000-8000-000000000002")
	postID        = uuid.MustParse("00000000-0000-4000-8000-000000000101")
	postTime      = time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	anonymous     = (*uuid.UUID)(nil)
	timedCursor   = &dto.TimedPostsCursor{PublishedOrUpdatedAt: postTime, PostID: postID}
	popularCursor = &dto.PopularPostsCursor{Likes: 10, PublishedAt: postTime, PostID: postID}
	postRequest   = dto.PostRequest{Title: "Мой первый пост", Body: "Текст поста", Status: "published"}
	postModel     = models.Post{Title: "Мой первый пост", Body: "Текст поста", Status: "published"}
)

type listMethod struct {
	name         string
	requiresAuth bool
	lookupAuthor bool
	nextCursor   func(p *dto.PostPreview) any
	expect       func(repo *mock_content_usecase.MockRepository, userID *uuid.UUID, items []*dto.PostPreview, err error)
	call         func(uc *postusecase.ContentUsecase, userID *uuid.UUID) (*page, error)
}

func listMethods() []listMethod {
	ctx := context.Background()
	return []listMethod{
		{
			name: "GetMyNewPosts", requiresAuth: true, nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetMyNewPosts(mock.Anything, u, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetMyNewPosts(ctx, u, timedCursor, pageLimit))
			},
		},
		{
			name: "GetMyOldPosts", requiresAuth: true, nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetMyOldPosts(mock.Anything, u, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetMyOldPosts(ctx, u, timedCursor, pageLimit))
			},
		},
		{
			name: "GetMyPopularPosts", requiresAuth: true, nextCursor: popularNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetMyPopularPosts(mock.Anything, u, popularCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromPopular(uc.GetMyPopularPosts(ctx, u, popularCursor, pageLimit))
			},
		},
		{
			name: "GetMyNewDrafts", requiresAuth: true, nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetMyNewDrafts(mock.Anything, u, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetMyNewDrafts(ctx, u, timedCursor, pageLimit))
			},
		},
		{
			name: "GetMyOldDrafts", requiresAuth: true, nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetMyOldDrafts(mock.Anything, u, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetMyOldDrafts(ctx, u, timedCursor, pageLimit))
			},
		},
		{
			name: "GetNewPosts", nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetNewPosts(mock.Anything, u, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetNewPosts(ctx, u, timedCursor, pageLimit))
			},
		},
		{
			name: "GetOldPosts", nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetOldPosts(mock.Anything, u, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetOldPosts(ctx, u, timedCursor, pageLimit))
			},
		},
		{
			name: "GetPopularPosts", nextCursor: popularNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				repo.EXPECT().GetPopularPosts(mock.Anything, u, popularCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromPopular(uc.GetPopularPosts(ctx, u, popularCursor, pageLimit))
			},
		},
		{
			name: "GetUserNewPosts", lookupAuthor: true, nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				authorID := postAuthorIDForItems(items)
				repo.EXPECT().GetUserNewPosts(mock.Anything, u, authorID, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetUserNewPosts(ctx, u, authorUsername, timedCursor, pageLimit))
			},
		},
		{
			name: "GetUserOldPosts", lookupAuthor: true, nextCursor: timedNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				authorID := postAuthorIDForItems(items)
				repo.EXPECT().GetUserOldPosts(mock.Anything, u, authorID, timedCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromTimed(uc.GetUserOldPosts(ctx, u, authorUsername, timedCursor, pageLimit))
			},
		},
		{
			name: "GetUserPopularPosts", lookupAuthor: true, nextCursor: popularNext,
			expect: func(repo *mock_content_usecase.MockRepository, u *uuid.UUID, items []*dto.PostPreview, err error) {
				authorID := postAuthorIDForItems(items)
				repo.EXPECT().GetUserPopularPosts(mock.Anything, u, authorID, popularCursor, pageLimit+1).Return(items, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (*page, error) {
				return fromPopular(uc.GetUserPopularPosts(ctx, u, authorUsername, popularCursor, pageLimit))
			},
		},
	}
}

func TestContentUsecase_ListPosts_Pagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fetched   int
		wantItems int
		wantNext  bool
	}{
		{name: "empty page", fetched: 0, wantItems: 0},
		{name: "last full page", fetched: pageLimit, wantItems: pageLimit},
		{name: "extra item becomes next cursor", fetched: pageLimit + 1, wantItems: pageLimit, wantNext: true},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo := mock_content_usecase.NewMockRepository(t)
				identity := mock_content_usecase.NewMockIdentityClient(t)

				items := previews(tt.fetched)
				wantItems := enrichedPreviews(items)

				m.expect(repo, &postUserID, items, nil)
				expectListIdentity(identity, m, items)

				uc := newTestContentUsecase(repo, identity)
				got, err := m.call(uc, &postUserID)

				require.NoError(t, err)
				require.Equal(t, wantItems[:tt.wantItems], got.items)
				if tt.wantNext {
					require.Equal(t, m.nextCursor(items[tt.wantItems-1]), got.next)
				} else {
					require.Nil(t, got.next)
				}
			})
		}
	}
}

func TestContentUsecase_ListPosts_FilteredPostsContinuePagination(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		for _, allFiltered := range []bool{false, true} {
			name := "some filtered"
			if allFiltered {
				name = "all filtered"
			}

			t.Run(m.name+"/"+name, func(t *testing.T) {
				t.Parallel()

				repo := mock_content_usecase.NewMockRepository(t)
				identity := mock_content_usecase.NewMockIdentityClient(t)
				items := previews(pageLimit + 1)
				missingAuthorID := uuid.New()
				for i := range items {
					if allFiltered || i == len(items)-1 {
						items[i].Author.ID = missingAuthorID
					}
				}

				m.expect(repo, &postUserID, items, nil)
				if m.lookupAuthor {
					identity.EXPECT().
						GetUserByUsername(mock.Anything, authorUsername).
						Return(identityUser(postAuthorIDForItems(items)), nil).
						Once()
				}

				users := map[uuid.UUID]*identitycontracts.UserInfo{}
				authorIDs := []uuid.UUID{postAuthorID, missingAuthorID}
				if allFiltered {
					authorIDs = []uuid.UUID{missingAuthorID}
				} else {
					users[postAuthorID] = identityUser(postAuthorID)
				}
				identity.EXPECT().
					GetUsers(mock.Anything, authorIDs).
					Return(users, nil).
					Once()

				uc := newTestContentUsecase(repo, identity)
				got, err := m.call(uc, &postUserID)

				require.NoError(t, err)
				if allFiltered {
					require.Empty(t, got.items)
				} else {
					require.Equal(t, enrichedPreviews(items[:pageLimit]), got.items)
				}
				require.Equal(t, m.nextCursor(items[len(items)-1]), got.next)
			})
		}
	}
}

func TestContentUsecase_ListPosts_Anonymous(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_content_usecase.NewMockRepository(t)
			identity := mock_content_usecase.NewMockIdentityClient(t)

			if !m.requiresAuth {
				items := previews(1)
				m.expect(repo, anonymous, items, nil)
				expectListIdentity(identity, m, items)
			}

			uc := newTestContentUsecase(repo, identity)
			got, err := m.call(uc, anonymous)

			if m.requiresAuth {
				require.Nil(t, got)
				require.ErrorIs(t, err, postusecase.ErrUnauthorized)

				return
			}
			require.NoError(t, err)
			require.Len(t, got.items, 1)
		})
	}
}

func TestContentUsecase_ListPosts_Errors(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("post storage unavailable")
	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "unknown author", repoErr: models.ErrPostNotFound, wantErr: postusecase.ErrNotFound},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, m := range listMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo := mock_content_usecase.NewMockRepository(t)
				identity := mock_content_usecase.NewMockIdentityClient(t)

				m.expect(repo, &postUserID, nil, tt.repoErr)
				expectListIdentity(identity, m, nil)

				uc := newTestContentUsecase(repo, identity)
				got, err := m.call(uc, &postUserID)

				require.Nil(t, got)
				require.ErrorIs(t, err, tt.wantErr)
			})
		}
	}
}

func TestContentUsecase_CreatePost_Success(t *testing.T) {
	t.Parallel()

	repo := mock_content_usecase.NewMockRepository(t)
	repo.EXPECT().CreatePost(mock.Anything, &postUserID, postModel).Return(postID, nil)

	uc := newTestContentUsecase(repo)
	got, err := uc.CreatePost(context.Background(), &postUserID, postRequest)

	require.NoError(t, err)
	require.Equal(t, postID, got)
}

func TestContentUsecase_CreatePost_Anonymous(t *testing.T) {
	t.Parallel()

	uc := newTestContentUsecase(mock_content_usecase.NewMockRepository(t))

	got, err := uc.CreatePost(context.Background(), anonymous, postRequest)

	require.Equal(t, uuid.Nil, got)
	require.ErrorIs(t, err, postusecase.ErrUnauthorized)
}

func TestContentUsecase_CreatePost_Errors(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("post storage unavailable")
	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "reader is not an author", repoErr: models.ErrInvalidAuthorID, wantErr: postusecase.ErrForbidden},
		{name: "invalid fields", repoErr: models.ErrPostValidation, wantErr: postusecase.ErrValidation},
		{name: "unauthorized in storage", repoErr: models.ErrUnauthorized, wantErr: postusecase.ErrUnauthorized},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_content_usecase.NewMockRepository(t)
			repo.EXPECT().CreatePost(mock.Anything, &postUserID, postModel).Return(uuid.Nil, tt.repoErr)

			uc := newTestContentUsecase(repo)
			got, err := uc.CreatePost(context.Background(), &postUserID, postRequest)

			require.Equal(t, uuid.Nil, got)
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestContentUsecase_GetPost(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("post storage unavailable")
	post := &dto.PostInfo{
		ID:     postID,
		Author: dto.AuthorResponse{ID: postAuthorID},
		Title:  "Мой первый пост",
	}
	tests := []struct {
		name    string
		userID  *uuid.UUID
		post    *dto.PostInfo
		repoErr error
		wantErr error
	}{
		{name: "authorized viewer", userID: &postUserID, post: post},
		{name: "anonymous viewer", userID: anonymous, post: post},
		{name: "hidden or unknown post", userID: &postUserID, repoErr: models.ErrPostNotFound, wantErr: postusecase.ErrNotFound},
		{name: "storage failure", userID: &postUserID, repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_content_usecase.NewMockRepository(t)
			identity := mock_content_usecase.NewMockIdentityClient(t)

			var repoPost *dto.PostInfo
			if tt.post != nil {
				postCopy := *tt.post
				repoPost = &postCopy
			}

			repo.EXPECT().
				GetPost(mock.Anything, tt.userID, postID).
				Return(repoPost, tt.repoErr)

			if tt.repoErr == nil {
				identity.EXPECT().
					GetUsers(mock.Anything, []uuid.UUID{postAuthorID}).
					Return(map[uuid.UUID]*identitycontracts.UserInfo{
						postAuthorID: identityUser(postAuthorID),
					}, nil).
					Once()
			}

			uc := newTestContentUsecase(repo, identity)
			got, err := uc.GetPost(context.Background(), tt.userID, postID)

			if tt.wantErr != nil {
				require.Nil(t, got)
				require.ErrorIs(t, err, tt.wantErr)

				return
			}

			require.NoError(t, err)
			require.Same(t, repoPost, got)
			require.Equal(t, dto.AuthorResponse{
				ID:       postAuthorID,
				Username: authorUsername,
				Nickname: "Иван",
			}, got.Author)
		})
	}
}

func TestContentUsecase_GetUserPosts_UnknownUsername(t *testing.T) {
	t.Parallel()

	for _, m := range listMethods() {
		if !m.lookupAuthor {
			continue
		}

		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_content_usecase.NewMockRepository(t)
			identity := mock_content_usecase.NewMockIdentityClient(t)

			identity.EXPECT().
				GetUserByUsername(mock.Anything, authorUsername).
				Return(&identitycontracts.UserInfo{}, identitycontracts.ErrUserNotFound).
				Once()

			uc := newTestContentUsecase(repo, identity)
			got, err := m.call(uc, &postUserID)

			require.Nil(t, got)
			require.ErrorIs(t, err, postusecase.ErrNotFound)
		})
	}
}

func TestContentUsecase_UpdatePost(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("post storage unavailable")
	updated := models.Post{ID: postID, Title: postModel.Title, Body: postModel.Body, Status: postModel.Status}
	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "actual version"},
		{name: "stale version", repoErr: models.ErrPostConflict, wantErr: postusecase.ErrConflict},
		{name: "foreign or unknown post", repoErr: models.ErrPostNotFound, wantErr: postusecase.ErrNotFound},
		{name: "invalid fields", repoErr: models.ErrPostValidation, wantErr: postusecase.ErrValidation},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_content_usecase.NewMockRepository(t)
			repo.EXPECT().UpdatePost(mock.Anything, &postUserID, updated, postTime).Return(tt.repoErr)

			uc := newTestContentUsecase(repo)
			err := uc.UpdatePost(context.Background(), &postUserID, postID, postRequest, postTime)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestContentUsecase_UpdatePost_Anonymous(t *testing.T) {
	t.Parallel()

	uc := newTestContentUsecase(mock_content_usecase.NewMockRepository(t))

	err := uc.UpdatePost(context.Background(), anonymous, postID, postRequest, postTime)

	require.ErrorIs(t, err, postusecase.ErrUnauthorized)
}

func TestContentUsecase_DeletePost(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("post storage unavailable")
	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{name: "own post"},
		{name: "foreign or unknown post", repoErr: models.ErrPostNotFound, wantErr: postusecase.ErrNotFound},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			repo := mock_content_usecase.NewMockRepository(t)
			repo.EXPECT().DeletePost(mock.Anything, &postUserID, postID).Return(tt.repoErr)

			uc := newTestContentUsecase(repo)
			err := uc.DeletePost(context.Background(), &postUserID, postID)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)

				return
			}
			require.NoError(t, err)
		})
	}
}

func TestContentUsecase_DeletePost_Anonymous(t *testing.T) {
	t.Parallel()

	uc := newTestContentUsecase(mock_content_usecase.NewMockRepository(t))

	err := uc.DeletePost(context.Background(), anonymous, postID)

	require.ErrorIs(t, err, postusecase.ErrUnauthorized)
}

type likeMethod struct {
	name   string
	expect func(repo *mock_content_usecase.MockRepository, likes int64, err error)
	call   func(uc *postusecase.ContentUsecase, userID *uuid.UUID) (int64, error)
}

func likeMethods() []likeMethod {
	ctx := context.Background()
	return []likeMethod{
		{
			name: "LikePost",
			expect: func(repo *mock_content_usecase.MockRepository, likes int64, err error) {
				repo.EXPECT().LikePost(mock.Anything, &postUserID, postID).Return(likes, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (int64, error) {
				return uc.LikePost(ctx, u, postID)
			},
		},
		{
			name: "DeletePostLike",
			expect: func(repo *mock_content_usecase.MockRepository, likes int64, err error) {
				repo.EXPECT().DeletePostLike(mock.Anything, &postUserID, postID).Return(likes, err)
			},
			call: func(uc *postusecase.ContentUsecase, u *uuid.UUID) (int64, error) {
				return uc.DeletePostLike(ctx, u, postID)
			},
		},
	}
}

func TestContentUsecase_Likes(t *testing.T) {
	t.Parallel()

	technicalErr := errors.New("post storage unavailable")
	tests := []struct {
		name      string
		repoLikes int64
		repoErr   error
		wantLikes int64
		wantErr   error
	}{
		{name: "returns actual likes count", repoLikes: 13, wantLikes: 13},
		{name: "unavailable post", repoErr: models.ErrPostNotFound, wantErr: postusecase.ErrNotFound},
		{name: "storage failure", repoErr: technicalErr, wantErr: technicalErr},
	}
	for _, m := range likeMethods() {
		for _, tt := range tests {
			t.Run(m.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				repo := mock_content_usecase.NewMockRepository(t)
				m.expect(repo, tt.repoLikes, tt.repoErr)

				uc := newTestContentUsecase(repo)
				got, err := m.call(uc, &postUserID)

				require.Equal(t, tt.wantLikes, got)
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)

					return
				}
				require.NoError(t, err)
			})
		}
	}
}

func TestContentUsecase_Likes_Anonymous(t *testing.T) {
	t.Parallel()

	for _, m := range likeMethods() {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()

			uc := newTestContentUsecase(mock_content_usecase.NewMockRepository(t))

			got, err := m.call(uc, anonymous)

			require.Zero(t, got)
			require.ErrorIs(t, err, postusecase.ErrUnauthorized)
		})
	}
}

func newTestContentUsecase(repo postusecase.Repository, identities ...postusecase.IdentityClient) *postusecase.ContentUsecase {
	s, err := sanitizer.NewSanitizer(sanitizer.DefaultConfig())
	if err != nil {
		panic(err)
	}

	var identity postusecase.IdentityClient
	if len(identities) > 0 {
		identity = identities[0]
	}

	return postusecase.New(repo, identity, s, testutils.SetupValidator(), "/media", testutils.SetupLogger())
}
