package release

import google_protobuf "github.com/golang/protobuf/ptypes/timestamp"

// Info describes release information.
type Info struct {
	Status        *Status                    `json:"status,omitempty"`
	FirstDeployed *google_protobuf.Timestamp `json:"first_deployed,omitempty"`
	LastDeployed  *google_protobuf.Timestamp `json:"last_deployed,omitempty"`
	// Deleted tracks when this object was deleted.
	Deleted *google_protobuf.Timestamp `json:"deleted,omitempty"`
	// Description is human-friendly "log entry" about this release.
	Description string `json:"Description,omitempty"`
}
