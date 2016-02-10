package kubectl

// Create uploads a chart to Kubernetes
func (r RealRunner) Create(stdin []byte) ([]byte, error) {
	args := []string{"create", "-f", "-"}

	cmd := command(args...)
	assignStdin(cmd, stdin)

	return cmd.CombinedOutput()
}

// Create returns the commands to kubectl
func (r PrintRunner) Create(stdin []byte) ([]byte, error) {
	args := []string{"create", "-f", "-"}

	cmd := command(args...)
	assignStdin(cmd, stdin)

	return []byte(cmd.String()), nil
}
