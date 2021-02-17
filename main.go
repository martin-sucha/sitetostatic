package main

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type queryParam struct {
	name string
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

func cacheKey(u url.URL) string {
	u.Fragment = ""
	u.RawFragment = ""
	var parts []queryParam
	for k, v := range u.Query() {
		parts = append(parts, queryParam{name: k, values: v})
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
}

func scrape(intialURL url.URL, workerCount int) {
	var client http.Client

	queue := []url.URL{intialURL}
	toScrape := make(chan url.URL)
	scraped := make(map[string]struct{})

	for i := 0; i < workerCount; i++ {
		go func() {

		}()
	}
}

func main() {

}
