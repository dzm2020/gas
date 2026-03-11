package strutil

import "unicode"

func IsFirstLetterUppercase(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		return unicode.IsLetter(r) && unicode.IsUpper(r)
	}
	return false
}
