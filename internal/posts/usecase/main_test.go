package postusecase_test

import (
	"time"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
	"github.com/google/uuid"
)

func previews(n int) []*dto.PostPreview {
	items := make([]*dto.PostPreview, n)
	for i := range items {
		publishedAt := postTime.Add(-time.Duration(i) * time.Hour)
		items[i] = &dto.PostPreview{
			ID:                   uuid.New(),
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
