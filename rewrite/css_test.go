package rewrite

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tdewolff/parse/v2"
)

func TestCSS(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output string
		skip   string
	}{
		{
			name:   "unquoted url",
			input:  "body { background: url(http://example.com/img.png); }",
			output: "body { background: url(\"https://example.net/newimg.png\"); }",
		},
		{
			name:   "unquoted url with spaces",
			input:  "body { background: url(  http://example.com/img.png   ); }",
			output: "body { background: url(  \"https://example.net/newimg.png\"   ); }",
		},
		{
			name:   "quoted url double",
			input:  "body { background: url(\"http://example.com/img.png\"); }",
			output: "body { background: url(\"https://example.net/newimg.png\"); }",
		},
		{
			name:   "quoted url single",
			input:  "body { background: url('http://example.com/img.png'); }",
			output: "body { background: url('https://example.net/newimg.png'); }",
		},
		{
			name:   "quoted url with spaces",
			input:  "body { background: url(  \"http://example.com/img.png\"   ); }",
			output: "body { background: url(  \"https://example.net/newimg.png\"   ); }",
		},
		{
			name:   "quoted url with ident url modifier",
			input:  "body { background: url(\"http://example.com/img.png\" fast); }",
			output: "body { background: url(\"https://example.net/newimg.png\" fast); }",
			skip:   "not supported by parse library",
		},
		{
			name:   "quoted url with functional url modifier",
			input:  "body { background: url(\"http://example.com/img.png\" fast(\"proxy\")); }",
			output: "body { background: url(\"https://example.net/newimg.png\" fast(\"proxy\"); }",
			skip:   "not supported by parse library",
		},
		{
			name:  "import string",
			input: "@import \"another.css\" print; body { background: url(\"http://example.com/img.png\"); }",
			output: "@import \"https://example.net/newimg.png\" print; " +
				"body { background: url(\"https://example.net/newimg.png\"); }",
		},
		{
			name:  "import url",
			input: "@import url(\"another.css\") print; body { background: url(\"http://example.com/img.png\"); }",
			output: "@import url(\"https://example.net/newimg.png\") print; " +
				"body { background: url(\"https://example.net/newimg.png\"); }",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name+" verbatim", func(t *testing.T) {
			if test.skip != "" {
				t.Skip(test.skip)
			}
			var sb strings.Builder
			rewriter := func(url URL) (string, error) {
				return "", ErrNotModified
			}
			err := CSS(parse.NewInputString(test.input), &sb, rewriter, false)
			if assert.NoError(t, err) {
				assert.Equal(t, test.input, sb.String())
			}
		})
		t.Run(test.name+" replaced", func(t *testing.T) {
			if test.skip != "" {
				t.Skip(test.skip)
			}
			var sb strings.Builder
			rewriter := func(url URL) (string, error) {
				return "https://example.net/newimg.png", nil
			}
			err := CSS(parse.NewInputString(test.input), &sb, rewriter, false)
			if assert.NoError(t, err) {
				assert.Equal(t, test.output, sb.String())
			}
		})
	}
}
