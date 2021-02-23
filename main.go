package main

import (
	"net/http"
	"net/url"
)

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
