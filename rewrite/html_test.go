package rewrite

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tdewolff/parse/v2"
)

func TestHTML5(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		inputFile   string
		output      string
		outputFile  string
		urlRewriter URLRewriter
		err         string
	}{
		{
			name:   "verbatim",
			input:  "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			output: "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return "", ErrNotModified
			},
			err: "",
		},
		{
			name:   "verbatim2",
			input:  "<html><body><input disabled ><a href = \"3.html\"></a></body></html>",
			output: "<html><body><input disabled ><a href = \"3.html\"></a></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return "", ErrNotModified
			},
			err: "",
		},
		{
			name:   "verbatim3",
			input:  "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			output: "<html   ><body><a href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return "", ErrNotModified
			},
			err: "",
		},
		{
			name:   "meta-refresh-verbatim",
			input:  "<html   ><head><meta content=\" 5;url=2.html\" http-equiv=\"refresh\"></head><body></body></html>",
			output: "<html   ><head><meta content=\" 5;url=2.html\" http-equiv=\"refresh\"></head><body></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return "", ErrNotModified

			},
			err: "",
		},
		{
			name:   "meta-refresh-rewrite",
			input:  "<html   ><head><meta content=\"5;url=2.html\" http-equiv=\"refresh\"></head><body></body></html>",
			output: "<html   ><head><meta content=\"5;url=REPLACED\" http-equiv=\"refresh\"></head><body></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "base-href-verbatim",
			input:  "<html   ><head><base href=\"http://example.com\"></head><body></body></html>",
			output: "<html   ><head><base href=\"http://example.com\"></head><body></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return "", ErrNotModified

			},
			err: "",
		},
		{
			name:   "base-href-rewrite",
			input:  "<html   ><head><base href=\"http://example.com\"></head><body></body></html>",
			output: "<html   ><head><base href=\"REPLACED\"></head><body></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "rewrite",
			input:  "<html   ><body><a   href=\"1&amp;.html\">1</a><a href='2.html'>1</a></body></html>",
			output: "<html   ><body><a   href=\"1&amp;.HTML.new\">1</a><a href='2.HTML.new'>1</a></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return strings.ToUpper(url.Value) + ".new", nil
			},
			err: "",
		},
		{
			name:   "rewrite multiple attributes",
			input:  "<html   ><body><a data=\"te'st\" href=\"1&amp;.html\">1</a><a href='2.\"html'>1</a></body></html>",
			output: "<html   ><body><a data=\"te'st\" href=\"1&amp;.HTML\">1</a><a href='2.\"HTML'>1</a></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return strings.ToUpper(url.Value), nil
			},
			err: "",
		},
		{
			name:   "style element",
			input:  "<html><head><style>@import \"a.html\"; body &gt; a { background: url(b.html) }</style></head></html>",
			output: "<html><head><style>@import \"A.HTML\"; body &gt; a { background: url(\"B.HTML\") }</style></head></html>",
			urlRewriter: func(url URL) (string, error) {
				return strings.ToUpper(url.Value), nil
			},
			err: "",
		},
		{
			name:   "style attribute",
			input:  "<html><body style=\"background: url('a.png')\"></body></html>",
			output: "<html><body style=\"background: url('A.PNG')\"></body></html>",
			urlRewriter: func(url URL) (string, error) {
				return strings.ToUpper(url.Value), nil
			},
			err: "",
		},
		{
			name:       "xhtml verbatim",
			inputFile:  "testdata/xhtml1.html",
			outputFile: "testdata/xhtml1.html",
			urlRewriter: func(url URL) (string, error) {
				return "", ErrNotModified
			},
			err: "",
		},
		{
			name:       "xhtml replaced",
			inputFile:  "testdata/xhtml1.html",
			outputFile: "testdata/xhtml1.replaced.html",
			urlRewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			inputData := []byte(test.input)
			if test.inputFile != "" {
				var err error
				inputData, err = ioutil.ReadFile(test.inputFile)
				require.NoError(t, err)
			}
			outputData := test.output
			if test.outputFile != "" {
				data, err := ioutil.ReadFile(test.outputFile)
				require.NoError(t, err)
				outputData = string(data)
			}
			input := parse.NewInputBytes(inputData)
			var output strings.Builder
			err := HTML5(input, &output, test.urlRewriter)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, outputData, output.String())
			}
		})
	}
}

func TestURLListAttribute(t *testing.T) {
	tests := []struct {
		name      string
		separator string
		input     string
		output    string
		rewriter  URLRewriter
		err       string
	}{
		{
			name:      "empty",
			separator: ",",
			input:     "",
			output:    "REPLACED",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:      "single",
			separator: ",",
			input:     "./test.html",
			output:    "REPLACED",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:      "multiple",
			separator: ",",
			input:     "./test.html, test2.html",
			output:    "REPLACED,REPLACED",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:      "multiple not modified",
			separator: ",",
			input:     "./test.html, test2.html",
			output:    "",
			rewriter: func(url URL) (string, error) {
				return "NOT_REPLACED", ErrNotModified
			},
			err: "not modified",
		},
		{
			name:      "multiple some modified",
			separator: ",",
			input:     "./test.html, test2.html",
			output:    "./test.html,REPLACED",
			rewriter: func(url URL) (string, error) {
				if url.Value == "./test.html" {
					return "", ErrNotModified
				}
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:      "custom error",
			separator: ",",
			input:     "./test.html, test2.html",
			output:    "",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", fmt.Errorf("custom error")
			},
			err: "custom error",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			fn := urlListAttribute(test.separator)
			lc := &html5Rewriter{urlRewriter: test.rewriter}
			output, err := fn(lc, test.input)
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
		name     string
		input    string
		output   string
		rewriter URLRewriter
		err      string
	}{
		{
			name:   "empty",
			input:  "",
			output: "REPLACED",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "not modified",
		},
		{
			name:   "single simple",
			input:  "./test.html",
			output: "REPLACED",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "single with options",
			input:  "./test.html 480w",
			output: "REPLACED 480w",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "multiple simple",
			input:  "./test.html, test2.html",
			output: "REPLACED, REPLACED",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "multiple with options",
			input:  "./test.html 480w, test2.html 870w",
			output: "REPLACED 480w, REPLACED 870w",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "multiple simple not modified",
			input:  "./test.html, test2.html",
			output: "",
			rewriter: func(url URL) (string, error) {
				return "NOT_REPLACED", ErrNotModified
			},
			err: "not modified",
		},
		{
			name:   "multiple with options not modified",
			input:  "./test.html 480w, test2.html 780w",
			output: "",
			rewriter: func(url URL) (string, error) {
				return "NOT_REPLACED", ErrNotModified
			},
			err: "not modified",
		},
		{
			name:   "multiple simple some modified",
			input:  "./test.html, test2.html",
			output: "./test.html, REPLACED",
			rewriter: func(url URL) (string, error) {
				if url.Value == "./test.html" {
					return "", ErrNotModified
				}
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "multiple with options some modified",
			input:  "./test.html 480w, test2.html 780w",
			output: "./test.html 480w, REPLACED 780w",
			rewriter: func(url URL) (string, error) {
				if url.Value == "./test.html" {
					return "", ErrNotModified
				}
				return "REPLACED", nil
			},
			err: "",
		},
		{
			name:   "custom error",
			input:  "./test.html, test2.html",
			output: "",
			rewriter: func(url URL) (string, error) {
				return "REPLACED", fmt.Errorf("custom error")
			},
			err: "custom error",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			lc := &html5Rewriter{urlRewriter: test.rewriter}
			output, err := srcSetAttribute(lc, test.input)
			if test.err != "" {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.output, output)
			}
		})
	}
}
