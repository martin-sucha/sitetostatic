package rewrite

import "errors"

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
)
