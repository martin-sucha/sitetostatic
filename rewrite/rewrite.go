package rewrite

import "errors"

// ErrNotModified can be returned by URLRewriter to not modify the URL.
var ErrNotModified = errors.New("not modified")

// URLRewriter is a function that can rewrite URL in documents.
// Return ErrNotModified if the URL should not be modified, this is faster than returning the same data.
type URLRewriter func(url string) (string, error)
