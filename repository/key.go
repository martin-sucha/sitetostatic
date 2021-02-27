package repository

import (
	"net/url"
	"sort"
	"strings"
)

// Key returns a canonical storage key for the given URL.
func Key(someURL *url.URL) string {
	// Per RFC3986, the canonical form of URL has:
	//
	//  - lowercase scheme
	//  - lowercase host / address
	//  - port omitted if default for scheme
	//  - colon between host:port not specified if port is empty
	//
	// On top of that, we:
	//
	//  - reorder query parameters
	//  - remove tracking query parameters
	//  - ignore fragment
	u := new(url.URL)
	*u = *someURL
	u.Scheme = strings.ToLower(u.Scheme)
	u.Fragment = ""
	u.RawFragment = ""
	host := u.Hostname()
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	u.Host = joinHostPort(strings.ToLower(host), port)
	if u.IsAbs() && u.Host != "" && u.Path == "" {
		u.Path = "/"
	}

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

func joinHostPort(host, port string) string {
	var sb strings.Builder
	// Assume IPv6 address if host contains colon.
	if strings.Contains(host, ":") {
		sb.WriteString("[")
		sb.WriteString(host)
		sb.WriteString("]")
	} else {
		sb.WriteString(host)
	}
	if port != "" {
		sb.WriteString(":")
		sb.WriteString(port)
	}
	return sb.String()
}
