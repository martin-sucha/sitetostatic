package apache2

import "strings"

// pcreEscaper escapes s to so that it's safe to embed inside PCRE expression.
// PCRE has more special characters than regexp module quotes.
// See https://www.php.net/manual/en/function.preg-quote.php
var pcreEscaper = strings.NewReplacer(
	`.`, `\.`,
	`\`, `\\`,
	`+`, `\+`,
	`*`, `\*`,
	`?`, `\?`,
	`[`, `\[`,
	`^`, `\^`,
	`]`, `\]`,
	`$`, `\$`,
	`(`, `\(`,
	`)`, `\)`,
	`{`, `\{`,
	`}`, `\}`,
	`=`, `\=`,
	`!`, `\!`,
	`<`, `\<`,
	`>`, `\>`,
	`|`, `\|`,
	`:`, `\:`,
	`-`, `\-`,
	`#`, `\#`,
)
