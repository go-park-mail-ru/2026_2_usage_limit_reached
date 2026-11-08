package feed

import "strconv"

func ParseLimit(raw string, defaultLimit, maxLimit int) int {
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > maxLimit {
		return defaultLimit
	}
	return limit
}

type Page[T, C any] struct {
	Items []T
	Next  *C
}

func Paginate[T, C any](items []T, limit int, cursorOf func(T) C) *Page[T, C] {
	if len(items) <= limit {
		return &Page[T, C]{Items: items}
	}
	items = items[:limit]
	next := cursorOf(items[limit-1])
	return &Page[T, C]{Items: items, Next: &next}
}
