package utils

import "github.com/gobwas/glob"

func MatchGlob(value string, pattern string) bool {
	matcher := glob.MustCompile(pattern)

	return matcher.Match(value)
}
