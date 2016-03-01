package kubectl

type TestRunner struct {
	Runner

	out []byte
	err error
}

func (r TestRunner) Get(stdin []byte, ns string) ([]byte, error) {
	return r.out, r.err
}
