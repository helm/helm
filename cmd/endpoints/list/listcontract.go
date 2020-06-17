package list

type ListRequest struct {
	RequestID string
}

type ListRespose struct {
	Status bool
	Data   []HelmRelease
}

type HelmRelease struct {
	Release   string `json:"release"`
	Namespace string `json:"namespace"`
}
