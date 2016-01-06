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
