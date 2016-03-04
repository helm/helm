package kubectl

// Get returns Kubernetes resources
func (r RealRunner) Get(stdin []byte, ns string) ([]byte, error) {
	args := []string{"get", "-f", "-"}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	assignStdin(cmd, stdin)

	return cmd.CombinedOutput()
}

// GetByKind gets resources by kind, name(optional), and namespace(optional)
func (r RealRunner) GetByKind(kind, name, ns string) (string, error) {
	args := []string{"get", kind}

	if name != "" {
		args = append([]string{name}, args...)
	}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	o, err := cmd.CombinedOutput()
	return string(o), err
}

// Get returns the commands to kubectl
func (r PrintRunner) Get(stdin []byte, ns string) ([]byte, error) {
	args := []string{"get", "-f", "-"}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	assignStdin(cmd, stdin)

	return []byte(cmd.String()), nil
}

// GetByKind gets resources by kind, name(optional), and namespace(optional)
func (r PrintRunner) GetByKind(kind, name, ns string) (string, error) {
	args := []string{"get", kind}

	if name != "" {
		args = append([]string{name}, args...)
	}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	cmd := command(args...)
	return cmd.String(), nil
}
