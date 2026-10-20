package i18n

import "os"

// Detect returns the user's system language, or Default if it is not
// supported. On Windows the preferred UI languages are consulted first;
// everywhere the POSIX locale variables are used (LC_ALL > LC_MESSAGES > LANG).
func Detect() Lang {
	candidates := platformLocales()
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		candidates = append(candidates, os.Getenv(env))
	}
	return detectFrom(candidates)
}

// detectFrom returns the first supported language among candidates.
func detectFrom(candidates []string) Lang {
	for _, c := range candidates {
		if l, ok := Parse(c); ok {
			return l
		}
	}
	return Default
}
