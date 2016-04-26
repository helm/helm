package lint

func All(basedir string) []Message {
	out := Chartfile(basedir)
	out = append(out, Templates(basedir)...)
	return out
}
