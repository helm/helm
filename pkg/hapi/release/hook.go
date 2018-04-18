package release

import "time"

type Hook_Event int32

const (
	Hook_UNKNOWN              Hook_Event = 0
	Hook_PRE_INSTALL          Hook_Event = 1
	Hook_POST_INSTALL         Hook_Event = 2
	Hook_PRE_DELETE           Hook_Event = 3
	Hook_POST_DELETE          Hook_Event = 4
	Hook_PRE_UPGRADE          Hook_Event = 5
	Hook_POST_UPGRADE         Hook_Event = 6
	Hook_PRE_ROLLBACK         Hook_Event = 7
	Hook_POST_ROLLBACK        Hook_Event = 8
	Hook_RELEASE_TEST_SUCCESS Hook_Event = 9
	Hook_RELEASE_TEST_FAILURE Hook_Event = 10
)

var Hook_Event_name = map[int32]string{
	0:  "UNKNOWN",
	1:  "PRE_INSTALL",
	2:  "POST_INSTALL",
	3:  "PRE_DELETE",
	4:  "POST_DELETE",
	5:  "PRE_UPGRADE",
	6:  "POST_UPGRADE",
	7:  "PRE_ROLLBACK",
	8:  "POST_ROLLBACK",
	9:  "RELEASE_TEST_SUCCESS",
	10: "RELEASE_TEST_FAILURE",
}
var Hook_Event_value = map[string]int32{
	"UNKNOWN":              0,
	"PRE_INSTALL":          1,
	"POST_INSTALL":         2,
	"PRE_DELETE":           3,
	"POST_DELETE":          4,
	"PRE_UPGRADE":          5,
	"POST_UPGRADE":         6,
	"PRE_ROLLBACK":         7,
	"POST_ROLLBACK":        8,
	"RELEASE_TEST_SUCCESS": 9,
	"RELEASE_TEST_FAILURE": 10,
}

func (x Hook_Event) String() string {
	return Hook_Event_name[int32(x)]
}

type Hook_DeletePolicy int32

const (
	Hook_SUCCEEDED            Hook_DeletePolicy = 0
	Hook_FAILED               Hook_DeletePolicy = 1
	Hook_BEFORE_HOOK_CREATION Hook_DeletePolicy = 2
)

var Hook_DeletePolicy_name = map[int32]string{
	0: "SUCCEEDED",
	1: "FAILED",
	2: "BEFORE_HOOK_CREATION",
}
var Hook_DeletePolicy_value = map[string]int32{
	"SUCCEEDED":            0,
	"FAILED":               1,
	"BEFORE_HOOK_CREATION": 2,
}

// Hook defines a hook object.
type Hook struct {
	Name string `json:"name,omitempty"`
	// Kind is the Kubernetes kind.
	Kind string `json:"kind,omitempty"`
	// Path is the chart-relative path to the template.
	Path string `json:"path,omitempty"`
	// Manifest is the manifest contents.
	Manifest string `json:"manifest,omitempty"`
	// Events are the events that this hook fires on.
	Events []Hook_Event `json:"events,omitempty"`
	// LastRun indicates the date/time this was last run.
	LastRun time.Time `json:"last_run,omitempty"`
	// Weight indicates the sort order for execution among similar Hook type
	Weight int32 `json:"weight,omitempty"`
	// DeletePolicies are the policies that indicate when to delete the hook
	DeletePolicies []Hook_DeletePolicy `json:"delete_policies,omitempty"`
}
