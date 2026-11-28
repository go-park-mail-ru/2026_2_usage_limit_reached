package repository

import (
	"strings"
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/internal/content/dto"
	"github.com/stretchr/testify/require"
)

func TestBuildFeedFilters_Empty(t *testing.T) {
	t.Parallel()

	sql, args, limitArg := buildFeedFilters(dto.FeedFilters{}, 4)

	require.Empty(t, sql)
	require.Empty(t, args)
	require.Equal(t, 4, limitArg)
}

// post_block после миграции 000013 хранит только media_id, тип вложения живёт
// в media.media_type, поэтому фильтр обязан ходить через JOIN media.
func TestBuildFeedFilters_AttachmentTypeUsesMediaTable(t *testing.T) {
	t.Parallel()

	sql, args, limitArg := buildFeedFilters(dto.FeedFilters{AttachmentType: dto.AttachmentTypeVideo}, 4)

	require.Contains(t, sql, "JOIN media m ON m.id = pb.media_id")
	require.Contains(t, sql, "$4 = 'any'")
	require.Contains(t, sql, "m.media_type = $4")
	require.NotContains(t, sql, "pb.media_type", "post_block больше не хранит media_type")
	require.Equal(t, []any{dto.AttachmentTypeVideo}, args)
	require.Equal(t, 5, limitArg)
}

func TestBuildFeedFilters_AttachmentAnyPassesThrough(t *testing.T) {
	t.Parallel()

	sql, args, _ := buildFeedFilters(dto.FeedFilters{AttachmentType: dto.AttachmentTypeAny}, 4)

	require.Contains(t, sql, "$4 = 'any'")
	require.Equal(t, []any{dto.AttachmentTypeAny}, args)
}

func TestBuildFeedFilters_PlaceholdersNumbering(t *testing.T) {
	t.Parallel()

	sql, args, limitArg := buildFeedFilters(dto.FeedFilters{
		Query:          "кот",
		Tag:            "Игры",
		AttachmentType: dto.AttachmentTypeVideo,
	}, 4)

	require.True(t, strings.HasPrefix(sql, "AND "))
	require.Contains(t, sql, "p.search_text ILIKE $4")
	require.Contains(t, sql, "a.category = $5")
	require.Contains(t, sql, "m.media_type = $6")
	require.Len(t, args, 3)
	require.Equal(t, 7, limitArg)
}

func TestBuildFeedFilters_EscapesLikeWildcards(t *testing.T) {
	t.Parallel()

	_, args, _ := buildFeedFilters(dto.FeedFilters{Query: `50%_off`}, 4)

	require.Equal(t, []any{`%50\%\_off%`}, args)
}
