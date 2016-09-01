package driver

import (
    "testing"

    rspb "k8s.io/helm/pkg/proto/hapi/release"
)

func TestRecordsAdd(t *testing.T) {
    rs := records([]*record{
        newRecord("rls-a.v1", releaseStub("rls-a", 1, rspb.Status_SUPERSEDED)),
        newRecord("rls-a.v2", releaseStub("rls-a", 2, rspb.Status_DEPLOYED)),
    })

    var tests = []struct {
        desc string
        key  string
        ok   bool
        rec  *record
    }{
        {
            "add valid key",
            "rls-a.v3",
            false,
            newRecord("rls-a.v3", releaseStub("rls-a", 3, rspb.Status_SUPERSEDED)),
        },
        {
            "add already existing key",
            "rls-a.v1",
            true,
            newRecord("rls-a.v1", releaseStub("rls-a", 1, rspb.Status_DEPLOYED)),
        },
    }

    for _, tt := range tests {
        if err := rs.Add(tt.rec); err != nil {
            if !tt.ok {
                t.Fatalf("failed: %q: %s\n", tt.desc, err)
            }
        }
    }
}

func TestRecordsRemove(t *testing.T) {
    var tests = []struct {
        desc string
        key  string
        ok   bool
    }{
        {"remove valid key", "rls-a.v1", false},
        {"remove invalid key", "rls-a.v", true},
        {"remove non-existant key", "rls-z.v1", true},
    }

    rs := records([]*record{
        newRecord("rls-a.v1", releaseStub("rls-a", 1, rspb.Status_SUPERSEDED)),
        newRecord("rls-a.v2", releaseStub("rls-a", 2, rspb.Status_DEPLOYED)),
    })

    for _, tt := range tests {
        if r := rs.Remove(tt.key); r == nil {
            if !tt.ok {
                t.Fatalf("Failed to %q (key = %s). Expected nil, got %s",
                    tt.desc,
                    tt.key,
                    r,
                )
            }
        }
    }
}
