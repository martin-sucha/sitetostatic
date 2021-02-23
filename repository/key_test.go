package repository

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKey(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		equal bool
	}{
		{
			name:  "empty",
			a:     "",
			b:     "",
			equal: true,
		},
		{
			name:  "same",
			a:     "https://example.com/a.html",
			b:     "https://example.com/a.html",
			equal: true,
		},
		{
			name:  "different port",
			a:     "https://example.com/a.html",
			b:     "https://example.com:444/a.html",
			equal: false,
		},
		{
			name:  "default port",
			a:     "https://example.com/a.html",
			b:     "https://example.com:443/a.html",
			equal: true,
		},
		{
			name:  "empty port",
			a:     "https://example.com/a.html",
			b:     "https://example.com:/a.html",
			equal: true,
		},
		{
			name:  "different host",
			a:     "https://example.com/a.html",
			b:     "https://example.net/a.html",
			equal: false,
		},
		{
			name:  "host is case insensitive",
			a:     "https://example.com/a.html",
			b:     "https://EXAMPLE.COM/a.html",
			equal: true,
		},
		{
			name:  "different scheme",
			a:     "https://example.com/a.html",
			b:     "http://example.com/a.html",
			equal: false,
		},
		{
			name:  "scheme is case insensitive",
			a:     "https://example.com/a.html",
			b:     "HTTPS://example.com/a.html",
			equal: true,
		},
		{
			name:  "different query",
			a:     "https://example.com/a.html?page=hello",
			b:     "https://example.com/a.html?page=world",
			equal: false,
		},
		{
			name:  "query param order does not matter",
			a:     "https://example.com/a.html?a=hello&b=world",
			b:     "https://example.com/a.html?b=world&a=hello",
			equal: true,
		},
		{
			name:  "order of multiple values of one param matters",
			a:     "https://example.com/a.html?a=hello&a=world",
			b:     "https://example.com/a.html?a=world&a=hello",
			equal: false,
		},
		{
			name:  "query params are case sensitive",
			a:     "https://example.com/a.html?a=hello",
			b:     "https://example.com/a.html?A=hello",
			equal: false,
		},
		{
			name:  "tracking params don't matter",
			a:     "https://example.com/a.html?utm_campaign=test&utm_source=example",
			b:     "https://example.com/a.html?utm_campaign=another&utm_source=search",
			equal: true,
		},
		{
			name:  "hash does not matter",
			a:     "https://example.com/a.html#hello",
			b:     "https://example.com/a.html#world",
			equal: true,
		},
		{
			name:  "hash does not matter 2",
			a:     "https://example.com/a.html#hello",
			b:     "https://example.com/a.html",
			equal: true,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			a, err := url.Parse(test.a)
			require.NoError(t, err)
			b, err := url.Parse(test.b)
			require.NoError(t, err)
			aKey := Key(a)
			bKey := Key(b)
			if test.equal {
				require.Equal(t, aKey, bKey)
			} else {
				require.True(t, aKey != bKey, aKey)
			}

		})
	}
}
