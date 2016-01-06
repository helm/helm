package kubectl

// Delete removes a chart from Kubernetes.
func (r RealRunner) Delete(name, ktype, ns string) ([]byte, error) {

	args := []string{"delete", ktype, name}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}
	return command(args...).CombinedOutput()
}

// Delete returns the commands to kubectl
func (r PrintRunner) Delete(name, ktype, ns string) ([]byte, error) {

	args := []string{"delete", ktype, name}

	if ns != "" {
		args = append([]string{"--namespace=" + ns}, args...)
	}

	cmd := command(args...)
	return []byte(cmd.String()), nil
}
