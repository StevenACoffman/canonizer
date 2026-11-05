package verify

// HeadingKey exposes headingKey to the black-box tests, for the one property that is about
// the key itself rather than about what Provenance reports.
func HeadingKey(s string) string { return headingKey(s) }
