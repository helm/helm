package kubectl

type TestRunner struct {
	Runner

	out []byte
	err error
}

func (r TestRunner) Get(stdin []byte, ns string) ([]byte, error) {
	return r.out, r.err
}

func (r TestRunner) GetByKind(kind, name, ns string) (string, error) {
	return string(r.out), r.err
}
