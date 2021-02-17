package rewrite

import (
	"fmt"
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
			name:        "verbatim3",
			input:       "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			output:      "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			urlRewriter: func(url string) (string, error) {
				return "", ErrNotModified
			},
			err:         "",
		},
		{
			name:        "rewrite",
			input:       "<html   ><body><a   href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			output:      "<html   ><body><a   href=\"1&amp;.HTML.new\">1</a><a href='2.HTML.new'>1</a></body></html>",
			urlRewriter: func(url string) (string, error) {
				return strings.ToUpper(url) + ".new", nil
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

func TestURLListAttribute(t *testing.T) {
	tests := []struct {
		name string
		separator string
		input string
		output string
		rewriter URLRewriter
		err string
	}{
		{
			name: "empty",
			separator: ",",
			input: "",
			output: "REPLACED",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "single",
			separator: ",",
			input: "./test.html",
			output: "REPLACED",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "multiple",
			separator: ",",
			input: "./test.html, test2.html",
			output: "REPLACED,REPLACED",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "multiple not modified",
			separator: ",",
			input: "./test.html, test2.html",
			output: "",
			rewriter: func(url string) (string, error) {
				return "NOT_REPLACED", ErrNotModified
			},
			err: "not modified",
		},
		{
			name: "multiple some modified",
			separator: ",",
			input: "./test.html, test2.html",
			output: "./test.html,REPLACED",
			rewriter: func(url string) (string, error) {
				if url == "./test.html" {
					return "", ErrNotModified
				}
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "custom error",
			separator: ",",
			input: "./test.html, test2.html",
			output: "",
			rewriter: func(url string) (string, error) {
				return "REPLACED", fmt.Errorf("custom error")
			},
			err: "custom error",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			fn := urlListAttribute(test.separator)
			output, err := fn(test.input, test.rewriter)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.output, output)
			}
		})
	}
}

func TestSrcSetAttribute(t *testing.T) {
	tests := []struct {
		name string
		input string
		output string
		rewriter URLRewriter
		err string
	}{
		{
			name: "empty",
			input: "",
			output: "REPLACED",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "not modified",
		},
		{
			name: "single simple",
			input: "./test.html",
			output: "REPLACED",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "single with options",
			input: "./test.html 480w",
			output: "REPLACED 480w",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "multiple simple",
			input: "./test.html, test2.html",
			output: "REPLACED, REPLACED",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "multiple with options",
			input: "./test.html 480w, test2.html 870w",
			output: "REPLACED 480w, REPLACED 870w",
			rewriter: func(url string) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "multiple simple not modified",
			input: "./test.html, test2.html",
			output: "",
			rewriter: func(url string) (string, error) {
				return "NOT_REPLACED", ErrNotModified
			},
			err: "not modified",
		},
		{
			name: "multiple with options not modified",
			input: "./test.html 480w, test2.html 780w",
			output: "",
			rewriter: func(url string) (string, error) {
				return "NOT_REPLACED", ErrNotModified
			},
			err: "not modified",
		},
		{
			name: "multiple simple some modified",
			input: "./test.html, test2.html",
			output: "./test.html, REPLACED",
			rewriter: func(url string) (string, error) {
				if url == "./test.html" {
					return "", ErrNotModified
				}
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "multiple with options some modified",
			input: "./test.html 480w, test2.html 780w",
			output: "./test.html 480w, REPLACED 780w",
			rewriter: func(url string) (string, error) {
				if url == "./test.html" {
					return "", ErrNotModified
				}
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name: "custom error",
			input: "./test.html, test2.html",
			output: "",
			rewriter: func(url string) (string, error) {
				return "REPLACED", fmt.Errorf("custom error")
			},
			err: "custom error",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			output, err := srcSetAttribute(test.input, test.rewriter)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.output, output)
			}
		})
	}
}
