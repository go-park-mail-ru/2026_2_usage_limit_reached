package cursor

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrInvalidCursor = errors.New("invalid cursor")

type CursorType string

type cursorGeneric struct {
	Kind    CursorType      `json:"kind"`
	Payload json.RawMessage `json:"payload"`
}

type Codec[T any] struct {
	Kind  CursorType
	Valid func(*T) bool
}

func (c Codec[T]) Encode(v *T) (*string, error) {
	if v == nil {
		return nil, nil
	}
	if !c.Valid(v) {
		return nil, ErrInvalidCursor
	}
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal cursor payload: %w", err)
	}
	raw, err := json.Marshal(cursorGeneric{Kind: c.Kind, Payload: payload})
	if err != nil {
		return nil, fmt.Errorf("marshal cursor: %w", err)
	}
	cur := base64.RawURLEncoding.EncodeToString(raw)
	return &cur, nil
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
	if err := strictUnmarshal(raw, &cursor); err != nil || cursor.Kind != c.Kind {
		return nil, ErrInvalidCursor
	}

	var v T
	if err := strictUnmarshal(cursor.Payload, &v); err != nil || !c.Valid(&v) {
		return nil, ErrInvalidCursor
	}
	return &v, nil
}

func strictUnmarshal(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
