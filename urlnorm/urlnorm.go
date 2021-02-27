package urlnorm

import (
	"net/url"
	"strings"
)

// Canonical returns the canonical form of URL.
//
// Per RFC3986, the canonical form of URL has:
//
//  - lowercase scheme
//  - lowercase host / address
//  - port omitted if default for scheme
//  - colon between host:port not specified if port is empty
//
// Canonical also ensures that path of absolute URLs always starts with /
func Canonical(someURL *url.URL) *url.URL {
	u := new(url.URL)
	*u = *someURL
	u.Scheme = strings.ToLower(u.Scheme)
	host := u.Hostname()
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}

	u.Host = joinHostPort(strings.ToLower(host), port)
	if u.IsAbs() && u.Host != "" && u.Path == "" {
		u.Path = "/"
	}
	return u
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
