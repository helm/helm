package upgrade

type UpgradeRequest struct {
	RequestID        string
	ReleaseName      string
	ReleaseNamespace string
	ChartPath        string
}

type UpgradeResponse struct {
	Status        bool
	ReleaseStatus string
}
