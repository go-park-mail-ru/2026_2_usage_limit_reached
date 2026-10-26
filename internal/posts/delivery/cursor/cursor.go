package cursor

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/posts/dto"
)

var ErrInvalidCursor = errors.New("invalid cursor")

type cursorType string

const (
	typeNewPosts     cursorType = "new_posts"
	typePopularPosts cursorType = "popular_posts"
)

type cursorGeneric struct {
	Kind    cursorType      `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type Codec[T any] struct {
	kind  cursorType
	valid func(*T) bool
}

var (
	NewPosts = Codec[dto.NewPostsCursor]{
		kind: typeNewPosts,
		valid: func(c *dto.NewPostsCursor) bool {
			return !c.PublishedAt.IsZero() && c.PostID != uuid.Nil
		},
	}
	PopularPosts = Codec[dto.PopularPostsCursor]{
		kind: typePopularPosts,
		valid: func(c *dto.PopularPostsCursor) bool {
			return c.Likes >= 0 && !c.PublishedAt.IsZero() && c.PostID != uuid.Nil
		},
	}
)

func (c Codec[T]) Encode(v *T) (string, error) {
	if v == nil {
		return "", nil
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal cursor payload: %w", err)
	}
	raw, err := json.Marshal(cursorGeneric{Kind: c.kind, Payload: payload})
	if err != nil {
		return "", fmt.Errorf("marshal cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (c Codec[T]) Decode(s string) (*T, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidCursor
	}

	var cursor cursorGeneric
	if err := strictUnmarshal(raw, &cursor); err != nil || cursor.Kind != c.kind {
		return nil, ErrInvalidCursor
	}

	var v T
	if err := strictUnmarshal(cursor.Payload, &v); err != nil || !c.valid(&v) {
		return nil, ErrInvalidCursor
	}
	return &v, nil
}

func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
