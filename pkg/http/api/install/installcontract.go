package install

type InstallRequest struct {
	RequestID        string
	ReleaseName      string
	ReleaseNamespace string
	ChartPath        string
}

type InstallReponse struct {
	Status        bool
	ReleaseStatus string
}
