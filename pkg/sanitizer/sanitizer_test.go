package sanitizer_test

import (
	"testing"

	"github.com/go-park-mail-ru/2026_2_usage_limit_reached/pkg/sanitizer"
	"github.com/stretchr/testify/require"
)

func newSanitizer(t *testing.T) *sanitizer.Sanitizer {
	t.Helper()

	s, err := sanitizer.NewSanitizer(sanitizer.DefaultConfig())
	require.NoError(t, err)

	return s
}

const linkAttrs = `target="_blank" rel="noopener noreferrer nofollow"`

func TestSanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Allowed tags are kept",
			input:    `<p>a <strong>b</strong> <em>c</em> <u>d</u> <s>e</s></p><h3>f</h3><ul><li>g</li></ul><ol><li>h</li></ol>i<br>j`,
			expected: `<p>a <strong>b</strong> <em>c</em> <u>d</u> <s>e</s></p><h3>f</h3><ul><li>g</li></ul><ol><li>h</li></ol>i<br/>j`,
		},
		{
			name:     "Tag aliases",
			input:    `<b>a</b><i>b</i><del>c</del><strike>d</strike><div>e</div>`,
			expected: `<strong>a</strong><em>b</em><s>c</s><s>d</s><p>e</p>`,
		},
		{
			name:     "Attributes are removed",
			input:    `<p style="color:red" class="c" id="i" onclick="alert(1)">a</p>`,
			expected: `<p>a</p>`,
		},
		{
			name:     "Https link is kept with target and rel",
			input:    `<a href=" https://example.com/path?q=1 " class="c" onclick="alert(1)">a</a>`,
			expected: `<a href="https://example.com/path?q=1" ` + linkAttrs + `>a</a>`,
		},
		{
			name:     "Quote in href does not break attribute",
			input:    `<a href="https://example.com/&quot;onmouseover=&quot;alert(1)">a</a>`,
			expected: `<a href="https://example.com/%22onmouseover=%22alert%281%29" ` + linkAttrs + `>a</a>`,
		},
		{
			name:     "Http link is unwrapped",
			input:    `<a href="http://example.com">a</a>`,
			expected: `a`,
		},
		{
			name:     "Relative link is unwrapped",
			input:    `<a href="/posts">a</a>`,
			expected: `a`,
		},
		{
			name:     "Javascript link is unwrapped",
			input:    `<a href=" JaVaScRiPt:alert(1)">a</a><a href="java&#x09;script:alert(1)">b</a>`,
			expected: `ab`,
		},
		{
			name:     "Data link is unwrapped",
			input:    `<a href="data:text/html,<script>alert(1)</script>">a</a>`,
			expected: `a`,
		},
		{
			name:     "Link without host or href is unwrapped",
			input:    `<a href="https:">a</a><a>b</a>`,
			expected: `ab`,
		},
		{
			name:     "Dangerous tags are removed with content",
			input:    `a<script>alert(1)</script><style>p{}</style><iframe>i</iframe><object>o</object><embed><template>t</template><noscript>n</noscript><svg onload="alert(1)"><text>s</text></svg><math><mi>m</mi></math><textarea>t</textarea><select><option>o</option></select>b`,
			expected: `ab`,
		},
		{
			name:     "Other tags are unwrapped",
			input:    `<p>a <span style="color:red">b <strong>c</strong></span> <font>d</font></p><section><h1>e</h1></section>`,
			expected: `<p>a b <strong>c</strong> d</p>e`,
		},
		{
			name:     "Images are removed",
			input:    `a<img src="x" onerror="alert(1)">b`,
			expected: `ab`,
		},
		{
			name:     "Comments are removed",
			input:    `a<!-- comment -->b`,
			expected: `ab`,
		},
		{
			name:     "Escaped tags stay text",
			input:    `&lt;script&gt;alert(1)&lt;/script&gt;`,
			expected: `&lt;script&gt;alert(1)&lt;/script&gt;`,
		},
		{
			name:     "Mutation XSS via noscript",
			input:    `<noscript><p title="</noscript><img src=x onerror=alert(1)>">`,
			expected: `&#34;&gt;`,
		},
		{
			name:     "Mutation XSS via math and style",
			input:    `<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>`,
			expected: ``,
		},
		{
			name:     "Empty input",
			input:    ``,
			expected: ``,
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

func TestHasText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "Text", input: `<p>a</p>`, expected: true},
		{name: "Escaped text", input: `<p>&amp;</p>`, expected: true},
		{name: "Empty", input: ``, expected: false},
		{name: "Only tags and spaces", input: `<p> </p><br/>`, expected: false},
		{name: "Only non-breaking space", input: `<p>&nbsp;</p>`, expected: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.expected, sanitizer.HasText(tt.input))
		})
	}
}
