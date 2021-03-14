# sitetostatic

Scrape a website so that it can be served from static files instead.

This is similar to [httrack](https://www.httrack.com/) and [wget -m](https://www.gnu.org/software/wget/),
but there are a few differences as neither tool did exactly what I wanted.
You might want to consider these alternatives for your use case.

I wanted to preserve original responses, including headers.

## How to scrape

```sh
sitetostatic scrape --allow-root http://example.com/ repository-path http://example.com/
sitetostatic files repository-path output-path
```

Similar result could be achieved with wget

```sh
cd output-path
wget -mpE http://example.com
```

but this does not preserve headers anywhere.

Alternately, you could use httrack to generate the files to serve:

```sh
httrack http://example.com/ -O output-path,repository-path -%v -k -%p -d -%q
```

with `-k` this stores all the files in the cache (`repository-path`).

However, the files in output-path don't contain original file extensions. For example if URL has
`file.aspx` and contains HTML, httrack outputs `file.html`, sitetostatic files outputs
`file.aspx.html`.

The `-%q` (`--include-query-string`) httrack options doesn't seem to work for me to include the query string the
filename.

## Verifying that you are serving the same data

There is a `sitetostatic diff` command to compare two repositories of scraped data (or httrack caches).
This is useful when you want to verify that the new web server returns the same data as the old site.
Just scrape also the new one and run `sitetostatic diff`.