package install

type InstallRequest struct {
	RequestID        string
	ReleaseName      string
	ReleaseNamespace string
	ChartPath        string
	Values           string
}

type InstallResponse struct {
	Status        bool
	ReleaseStatus string
}
