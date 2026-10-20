//go:build windows

package i18n

import "golang.org/x/sys/windows"

// platformLocales returns the user's preferred UI languages, e.g. "de-DE".
func platformLocales() []string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil {
		return nil
	}
	return langs
}
