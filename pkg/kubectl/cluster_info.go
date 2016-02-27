package kubectl

// ClusterInfo returns Kubernetes cluster info
func (r RealRunner) ClusterInfo() ([]byte, error) {
	return command("cluster-info").CombinedOutput()
}

// ClusterInfo returns the commands to kubectl
func (r PrintRunner) ClusterInfo() ([]byte, error) {
	cmd := command("cluster-info")
	return []byte(cmd.String()), nil
}
