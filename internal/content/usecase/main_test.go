package postusecase_test

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/usecase/mock_content_usecase"
	identitycontracts "github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/contracts/identity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

func previews(n int) []*dto.PostPreview {
	items := make([]*dto.PostPreview, n)
	for i := range items {
		publishedAt := postTime.Add(-time.Duration(i) * time.Hour)
		items[i] = &dto.PostPreview{
			ID:                   uuid.New(),
			Author:               dto.AuthorResponse{ID: postAuthorID},
			Title:                "Пост",
			PublishedOrUpdatedAt: &publishedAt,
			LikesCount:           int64(100 - i),
		}
	}
	return items
}

type page struct {
	items []*dto.PostPreview
	next  any
}

func fromTimed(p *dto.TimedPostsPage, err error) (*page, error) {
	if p == nil {
		return nil, err
	}
	res := &page{items: p.Items}
	if p.Next != nil {
		res.next = p.Next
	}
	return res, err
}

func fromPopular(p *dto.PopularPostsPage, err error) (*page, error) {
	if p == nil {
		return nil, err
	}
	res := &page{items: p.Items}
	if p.Next != nil {
		res.next = p.Next
	}
	return res, err
}

func timedNext(p *dto.PostPreview) any {
	return &dto.TimedPostsCursor{PublishedOrUpdatedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
}

func popularNext(p *dto.PostPreview) any {
	return &dto.PopularPostsCursor{Likes: p.LikesCount, PublishedAt: *p.PublishedOrUpdatedAt, PostID: p.ID}
}

func identityUser(id uuid.UUID) *identitycontracts.UserInfo {
	return &identitycontracts.UserInfo{
		ID:       id,
		Username: authorUsername,
		Nickname: "Иван",
	}
}

func postAuthorIDForItems(items []*dto.PostPreview) uuid.UUID {
	if len(items) == 0 {
		return postAuthorID
	}
	return items[0].Author.ID
}

func expectGetUsers(identity *mock_content_usecase.MockIdentityClient, items []*dto.PostPreview) {
	if len(items) == 0 {
		return
	}

	ids := make([]uuid.UUID, 0, len(items))
	users := make(map[uuid.UUID]*identitycontracts.UserInfo, len(items))
	seen := make(map[uuid.UUID]struct{}, len(items))

	for _, item := range items {
		id := item.Author.ID
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		ids = append(ids, id)
		users[id] = identityUser(id)
	}

	identity.EXPECT().
		GetUsers(mock.Anything, ids).
		Return(users, nil).
		Once()
}

func expectListIdentity(
	identity *mock_content_usecase.MockIdentityClient,
	m listMethod,
	items []*dto.PostPreview,
) {
	if m.lookupAuthor {
		authorID := postAuthorIDForItems(items)

		identity.EXPECT().
			GetUserByUsername(mock.Anything, authorUsername).
			Return(identityUser(authorID), nil).
			Once()
	}

	expectGetUsers(identity, items)
}

func enrichedPreviews(items []*dto.PostPreview) []*dto.PostPreview {
	result := make([]*dto.PostPreview, len(items))

	for i, item := range items {
		copy := *item
		copy.Author = dto.AuthorResponse{
			ID:       item.Author.ID,
			Username: authorUsername,
			Nickname: "Иван",
		}
		result[i] = &copy
	}

	return result
}
