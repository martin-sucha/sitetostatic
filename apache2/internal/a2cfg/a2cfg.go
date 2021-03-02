package a2cfg

type Config struct {
	VirtualHosts []*VirtualHost
}

// VirtualHost directive.
// https://httpd.apache.org/docs/2.4/mod/core.html#virtualhost
type VirtualHost struct {
	ServerName      string
	Port            string
	Aliases         []*Alias
	RedirectMatches []*RedirectMatch
}

// Alias directive.
// https://httpd.apache.org/docs/2.4/mod/mod_alias.html#alias
type Alias struct {
	URLPath  string
	FilePath string
}

// RedirectMatch directive.
// https://httpd.apache.org/docs/2.4/mod/mod_alias.html#redirectmatch
type RedirectMatch struct {
	Status int
	Regex  string
	URL    string
}
