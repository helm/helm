package release

type Status_Code int32

const (
	// Status_UNKNOWN indicates that a release is in an uncertain state.
	Status_UNKNOWN Status_Code = 0
	// Status_DEPLOYED indicates that the release has been pushed to Kubernetes.
	Status_DEPLOYED Status_Code = 1
	// Status_DELETED indicates that a release has been deleted from Kubermetes.
	Status_DELETED Status_Code = 2
	// Status_SUPERSEDED indicates that this release object is outdated and a newer one exists.
	Status_SUPERSEDED Status_Code = 3
	// Status_FAILED indicates that the release was not successfully deployed.
	Status_FAILED Status_Code = 4
	// Status_DELETING indicates that a delete operation is underway.
	Status_DELETING Status_Code = 5
	// Status_PENDING_INSTALL indicates that an install operation is underway.
	Status_PENDING_INSTALL Status_Code = 6
	// Status_PENDING_UPGRADE indicates that an upgrade operation is underway.
	Status_PENDING_UPGRADE Status_Code = 7
	// Status_PENDING_ROLLBACK indicates that an rollback operation is underway.
	Status_PENDING_ROLLBACK Status_Code = 8
)

var Status_Code_name = map[int32]string{
	0: "UNKNOWN",
	1: "DEPLOYED",
	2: "DELETED",
	3: "SUPERSEDED",
	4: "FAILED",
	5: "DELETING",
	6: "PENDING_INSTALL",
	7: "PENDING_UPGRADE",
	8: "PENDING_ROLLBACK",
}
var Status_Code_value = map[string]int32{
	"UNKNOWN":          0,
	"DEPLOYED":         1,
	"DELETED":          2,
	"SUPERSEDED":       3,
	"FAILED":           4,
	"DELETING":         5,
	"PENDING_INSTALL":  6,
	"PENDING_UPGRADE":  7,
	"PENDING_ROLLBACK": 8,
}

func (x Status_Code) String() string {
	return Status_Code_name[int32(x)]
}

// Status defines the status of a release.
type Status struct {
	Code Status_Code `json:"code,omitempty"`
	// Cluster resources as kubectl would print them.
	Resources string `json:"resources,omitempty"`
	// Contains the rendered templates/NOTES.txt if available
	Notes string `json:"notes,omitempty"`
	// LastTestSuiteRun provides results on the last test run on a release
	LastTestSuiteRun *TestSuite `json:"last_test_suite_run,omitempty"`
}
