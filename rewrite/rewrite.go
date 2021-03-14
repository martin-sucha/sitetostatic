package rewrite

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tdewolff/parse/v2"
)

// ErrNotModified can be returned by URLRewriter to not modify the URL.
var ErrNotModified = errors.New("not modified")

// URLRewriter is a function that can rewrite URL in documents.
// Return ErrNotModified if the URL should not be modified, this is faster than returning the same data.
type URLRewriter func(url URL) (string, error)

type URL struct {
	// Value is the original URL.
	Value string
	// Base is original base URL.
	// Empty if rewriting the base URL itself.
	Base string
	// NewBase is the new base URL.
	// Empty if rewriting the base URL itself.
	NewBase string
	// Type of the URL.
	Type URLType
}

type URLType uint8

const (
	URLTypeUnknown URLType = iota
	URLTypeBase
	URLTypeOpenGraph
	URLTypeCSS
)

// IsSupportedMediaType returns whether the given media type (as returned from mime.ParseMediaType) is supported.
func IsSupportedMediaType(mediaType string, params map[string]string) bool {
	if mediaType != "text/html" && mediaType != "text/css" {
		return false
	}
	return params["charset"] == "" || strings.EqualFold(params["charset"], "utf-8")
}

// Document rewrites whole document by given MIME media type.
func Document(mediaType string, mediaParams map[string]string, input *parse.Input, w io.Writer,
	urlRewriter URLRewriter) error {
	if !IsSupportedMediaType(mediaType, mediaParams) {
		return fmt.Errorf("unsupported media type: %s %v", mediaType, mediaParams)
	}

	switch mediaType {
	case "text/html":
		return HTML5(input, w, urlRewriter)
	case "text/css":
		return CSS(input, w, urlRewriter, false)
	default:
		return fmt.Errorf("unsupported media type: %s %v", mediaType, mediaParams)
	}
}
