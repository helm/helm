package release

type StatusCode int

const (
	// Status_UNKNOWN indicates that a release is in an uncertain state.
	Status_UNKNOWN StatusCode = iota
	// Status_DEPLOYED indicates that the release has been pushed to Kubernetes.
	Status_DEPLOYED
	// Status_DELETED indicates that a release has been deleted from Kubermetes.
	Status_DELETED
	// Status_SUPERSEDED indicates that this release object is outdated and a newer one exists.
	Status_SUPERSEDED
	// Status_FAILED indicates that the release was not successfully deployed.
	Status_FAILED
	// Status_DELETING indicates that a delete operation is underway.
	Status_DELETING
	// Status_PENDING_INSTALL indicates that an install operation is underway.
	Status_PENDING_INSTALL
	// Status_PENDING_UPGRADE indicates that an upgrade operation is underway.
	Status_PENDING_UPGRADE
	// Status_PENDING_ROLLBACK indicates that an rollback operation is underway.
	Status_PENDING_ROLLBACK
)

var statusCodeNames = [...]string{
	"UNKNOWN",
	"DEPLOYED",
	"DELETED",
	"SUPERSEDED",
	"FAILED",
	"DELETING",
	"PENDING_INSTALL",
	"PENDING_UPGRADE",
	"PENDING_ROLLBACK",
}

func (x StatusCode) String() string { return statusCodeNames[x] }

// Status defines the status of a release.
type Status struct {
	Code StatusCode `json:"code,omitempty"`
	// Cluster resources as kubectl would print them.
	Resources string `json:"resources,omitempty"`
	// Contains the rendered templates/NOTES.txt if available
	Notes string `json:"notes,omitempty"`
	// LastTestSuiteRun provides results on the last test run on a release
	LastTestSuiteRun *TestSuite `json:"last_test_suite_run,omitempty"`
}
