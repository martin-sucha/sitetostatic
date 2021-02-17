package rewrite

import (
	"github.com/stretchr/testify/assert"
	"github.com/tdewolff/parse/v2"
	"strings"
	"testing"
)

func TestHTML5(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		output      string
		urlRewriter URLRewriter
		err string
	}{
		{
			name:        "verbatim",
			input:       "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			output:      "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			urlRewriter: func(url string) (string, error) {
				return "", ErrNotModified
			},
			err:         "",
		},
		{
			name:        "verbatim2",
			input:       "<html><body><input disabled ><a href = \"3.html\"></a></body></html>",
			output:      "<html><body><input disabled ><a href = \"3.html\"></a></body></html>",
			urlRewriter: func(url string) (string, error) {
				return "", ErrNotModified
			},
			err:         "",
		},
		{
			name:        "rewrite",
			input:       "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			output:      "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			urlRewriter: func(url string) (string, error) {
				return "", ErrNotModified
			},
			err:         "",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			input := parse.NewInputString(test.input)
			var output strings.Builder
			err := HTML5(input, &output, test.urlRewriter)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.output, output.String())
			}
		})
	}
}
