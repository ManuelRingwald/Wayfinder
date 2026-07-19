package basemapsearch

import "strings"

// normalizeName folds a display name (or a query) into the comparable search
// form: lower-case, German umlauts/ß expanded, the street suffix unified to
// "str" (so "Friedrichstraße", "Friedrichstrasse" and "Friedrichstr." all meet
// a query for any of the three), whitespace collapsed. Deliberately simple —
// this is a sector index, not a full geocoder; typo tolerance is out of scope
// (#277 honest limit).
func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer(
		"ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
		"straße", "str", "strasse", "str", "str.", "str",
	)
	s = r.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}
