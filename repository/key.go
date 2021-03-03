package repository

import (
	"net/url"
	"sort"
	"strings"

	"github.com/martin-sucha/site-to-static/urlnorm"
)

// Key returns a canonical storage key for the given URL.
// Applies changes from urlnorm.Canonical and on top of that, we:
//
//  - reorder query parameters
//  - remove tracking query parameters
//  - ignore fragment
func Key(someURL *url.URL) string {
	u := urlnorm.Canonical(someURL)

	var parts []queryParam
	for k, v := range u.Query() {
		switch k {
		case "utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content":
			// ignore
		default:
			parts = append(parts, queryParam{name: k, values: v})
		}
	}
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].name < parts[j].name
	})
	var rawQuery strings.Builder
	for i, part := range parts {
		if i > 0 {
			rawQuery.WriteString("&")
		}
		rawQuery.WriteString(part.String())
	}
	u.RawQuery = rawQuery.String()
	u.Fragment = ""
	u.RawFragment = ""
	return u.String()
}

type queryParam struct {
	name   string
	values []string
}

func (q queryParam) String() string {
	var sb strings.Builder
	sb.WriteString(url.QueryEscape(q.name))
	sb.WriteString("=")
	for i, val := range q.values {
		if i > 0 {
			sb.WriteString("&")
			sb.WriteString(url.QueryEscape(q.name))
			sb.WriteString("=")
		}
		sb.WriteString(url.QueryEscape(val))
	}
	return sb.String()
}
