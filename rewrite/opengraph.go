package rewrite

var openGraphURLProperties = map[string]struct{}{
	"image":               {},
	"og:url":              {},
	"og:image":            {},
	"og:image:url":        {},
	"og:image:secure_url": {},
	"og:video":            {},
	"og:video:url":        {},
	"og:video:secure_url": {},
	"og:audio":            {},
	"og:audio:url":        {},
	"og:audio:secure_url": {},
}

func isOpenGraphURLProperty(name string) bool {
	_, ok := openGraphURLProperties[name]
	return ok
}
