package sanitizer_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	firstAttachmentID  = "00000000-0000-4000-8000-000000000301"
	secondAttachmentID = "00000000-0000-4000-8000-000000000302"
)

func TestSanitize_FigureMarker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "marker is kept",
			input:    `<p>a</p><figure data-attachment-id="` + firstAttachmentID + `"></figure>`,
			expected: `<p>a</p><figure data-attachment-id="` + firstAttachmentID + `"></figure>`,
		},
		{
			name:     "extra attributes are dropped",
			input:    `<figure class="c" style="x" onclick="alert(1)" data-attachment-id="` + firstAttachmentID + `"></figure>`,
			expected: `<figure data-attachment-id="` + firstAttachmentID + `"></figure>`,
		},
		{
			name:     "marker without valid id is unwrapped",
			input:    `<p>a</p><figure data-attachment-id="not-a-uuid"></figure>`,
			expected: `<p>a</p>`,
		},
		{
			name:     "marker without id is unwrapped",
			input:    `<p>a</p><figure></figure>`,
			expected: `<p>a</p>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newSanitizer(t).Sanitize(tt.input)

			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}

func TestAttachmentIDs(t *testing.T) {
	t.Parallel()

	var (
		first  = uuid.MustParse(firstAttachmentID)
		second = uuid.MustParse(secondAttachmentID)
	)
	tests := []struct {
		name     string
		input    string
		expected []uuid.UUID
	}{
		{name: "no markers", input: `<p>Текст поста</p>`},
		{
			name:     "markers in document order",
			input:    `<p>a</p><figure data-attachment-id="` + secondAttachmentID + `"></figure><p>b</p><figure data-attachment-id="` + firstAttachmentID + `"></figure>`,
			expected: []uuid.UUID{second, first},
		},
		{
			name:     "repeated marker returns id twice",
			input:    `<figure data-attachment-id="` + firstAttachmentID + `"></figure><figure data-attachment-id="` + firstAttachmentID + `"></figure>`,
			expected: []uuid.UUID{first, first},
		},
		{
			name:     "invalid id is skipped",
			input:    `<figure data-attachment-id="not-a-uuid"></figure><figure data-attachment-id="` + firstAttachmentID + `"></figure>`,
			expected: []uuid.UUID{first},
		},
		{
			name:     "nested marker is extracted",
			input:    `<p><figure data-attachment-id="` + firstAttachmentID + `"></figure></p>`,
			expected: []uuid.UUID{first},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := newSanitizer(t).AttachmentIDs(tt.input)

			require.NoError(t, err)
			require.Equal(t, tt.expected, got)
		})
	}
}
