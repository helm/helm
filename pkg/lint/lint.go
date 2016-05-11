package lint

// All runs all of the available linters on the given base directory.
func All(basedir string) []Message {
	out := Chartfile(basedir)
	out = append(out, Templates(basedir)...)
	out = append(out, Values(basedir)...)
	return out
}
