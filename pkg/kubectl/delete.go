package kubectl

// Delete removes a chart from Kubernetes.
func (r RealRunner) Delete(name, ktype string) ([]byte, error) {

	args := []string{"delete", ktype, name}

	return command(args...).CombinedOutput()
}

// Delete returns the commands to kubectl
func (r PrintRunner) Delete(name, ktype string) ([]byte, error) {

	args := []string{"delete", ktype, name}

	cmd := command(args...)
	return []byte(cmd.String()), nil
}
