package release

import "time"

type HookEvent int

const (
	Hook_UNKNOWN HookEvent = iota
	Hook_PRE_INSTALL
	Hook_POST_INSTALL
	Hook_PRE_DELETE
	Hook_POST_DELETE
	Hook_PRE_UPGRADE
	Hook_POST_UPGRADE
	Hook_PRE_ROLLBACK
	Hook_POST_ROLLBACK
	Hook_RELEASE_TEST_SUCCESS
	Hook_RELEASE_TEST_FAILURE
)

var eventNames = [...]string{
	"UNKNOWN",
	"PRE_INSTALL",
	"POST_INSTALL",
	"PRE_DELETE",
	"POST_DELETE",
	"PRE_UPGRADE",
	"POST_UPGRADE",
	"PRE_ROLLBACK",
	"POST_ROLLBACK",
	"RELEASE_TEST_SUCCESS",
	"RELEASE_TEST_FAILURE",
}

func (x HookEvent) String() string { return eventNames[x] }

type HookDeletePolicy int

const (
	Hook_SUCCEEDED HookDeletePolicy = iota
	Hook_FAILED
	Hook_BEFORE_HOOK_CREATION
)

var deletePolicyNames = [...]string{
	"SUCCEEDED",
	"FAILED",
	"BEFORE_HOOK_CREATION",
}

func (x HookDeletePolicy) String() string { return deletePolicyNames[x] }

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
	Events []HookEvent `json:"events,omitempty"`
	// LastRun indicates the date/time this was last run.
	LastRun time.Time `json:"last_run,omitempty"`
	// Weight indicates the sort order for execution among similar Hook type
	Weight int `json:"weight,omitempty"`
	// DeletePolicies are the policies that indicate when to delete the hook
	DeletePolicies []HookDeletePolicy `json:"delete_policies,omitempty"`
}
