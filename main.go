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
			fmt.Println(doc.Metadata.URL)
			_ = doc.Close()
		}
	case "httrack":
		cache, err := httrack.OpenCache(repoPath)
		if err != nil {
			return err
		}
		for _, entry := range cache.Entries {
			fmt.Println(entry.URL)
		}
	}

	return nil
}
