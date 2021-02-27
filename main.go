package main

import (
	"fmt"
	"net/url"
	"os"
	"site-to-static/httrack"
	"site-to-static/repository"
	"site-to-static/scraper"
	"strings"

	"github.com/urfave/cli/v2"
	"golang.org/x/time/rate"
)

func main() {
	app := &cli.App{
		Name:  "scrape-to-static",
		Usage: "Scrape a website and convert it to a static site",
		Commands: []*cli.Command{
			{
				Name:      "scrape",
				Usage:     "",
				ArgsUsage: "repopath url [url...]",
				Action:    doScrape,
				Flags: []cli.Flag{
					&cli.StringSliceFlag{
						Name:  "allow-root",
						Usage: "URL prefixes to allow",
					},
				},
			},
			{
				Name:      "list",
				Usage:     "list urls stored in a repository",
				ArgsUsage: "repopath",
				Action:    doList,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "format",
						Usage: "either native or httrack",
					},
					&cli.BoolFlag{
						Name:  "canonical",
						Usage: "print canonical URLs",
					},
				},
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func doScrape(c *cli.Context) error {
	if c.Args().Len() < 2 {
		return fmt.Errorf("not enough arguments")
	}
	repoPath := c.Args().First()
	initialURLArgs := c.Args().Tail()
	initialURLs := make([]*url.URL, 0, len(initialURLArgs))
	for _, arg := range initialURLArgs {
		u, err := url.Parse(arg)
		if err != nil {
			return fmt.Errorf("parse initial url %q: %v", arg, err)
		}
		initialURLs = append(initialURLs, u)
	}

	rootStrings := c.StringSlice("allow-root")
	rootKeys := make([]string, 0, len(rootStrings))
	for _, arg := range rootStrings {
		u, err := url.Parse(arg)
		if err != nil {
			return fmt.Errorf("parse root url %q: %v", arg, err)
		}
		rootKeys = append(rootKeys, repository.Key(u))
	}

	repo := repository.New(repoPath)
	sc := scraper.Scraper{
		Repository: repo,
		Limiter:    rate.NewLimiter(10, 1),
		FollowURL: func(u *url.URL) bool {
			key := repository.Key(u)
			for _, root := range rootKeys {
				if strings.HasPrefix(key, root) {
					return true
				}
			}
			return false
		},
	}
	sc.Scrape(initialURLs, 10)
	return nil
}

func doList(c *cli.Context) error {
	if c.Args().Len() < 1 {
		return fmt.Errorf("not enough arguments")
	}
	format := c.String("format")

	printURLFunc := func(u string) error {
		_, err := fmt.Println(u)
		return err
	}
	if c.Bool("canonical") {
		printURLFunc = func(u string) error {
			parsedURL, err := url.Parse(u)
			if err != nil {
				return err
			}
			_, err = fmt.Println(canonicalURL(parsedURL))
			return err
		}
	}

	repoPath := c.Args().First()
	switch format {
	case "", "native":
		repo := repository.New(repoPath)
		entries, err := repo.List()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			doc, err := entry.Open()
			if err != nil {
				return err
			}
			err = printURLFunc(doc.Metadata.URL)
			if err != nil {
				return err
			}
			err = doc.Close()
			if err != nil {
				return err
			}
		}
	case "httrack":
		cache, err := httrack.OpenCache(repoPath)
		if err != nil {
			return err
		}
		for _, entry := range cache.Entries {
			err = printURLFunc(entry.URL)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// canonicalURL returns the canonical form of URL.
func canonicalURL(someURL *url.URL) string {
	// Per RFC3986, the canonical form of URL has:
	//
	//  - lowercase scheme
	//  - lowercase host / address
	//  - port omitted if default for scheme
	//  - colon between host:port not specified if port is empty
	u := new(url.URL)
	*u = *someURL
	u.Scheme = strings.ToLower(u.Scheme)
	host := u.Hostname()
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}

	u.Host = joinHostPort(strings.ToLower(host), port)
	return u.String()
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
