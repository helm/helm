package driver

import (
    "testing"
)

func TestLabelsMatch(t *testing.T) {
    var tests = []struct {
        desc   string
        set1   labels
        set2   labels
        expect bool
    }{
        {
            "equal labels sets",
            labels(map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}),
            labels(map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}),
            true,
        },
        {
            "disjoint label sets",
            labels(map[string]string{"KEY_C": "VAL_C", "KEY_D": "VAL_D"}),
            labels(map[string]string{"KEY_A": "VAL_A", "KEY_B": "VAL_B"}),
            false,
        },
    }

    for _, tt := range tests {
        if !tt.set1.match(tt.set2) && tt.expect {
            t.Fatalf("Expected match '%s'\n", tt.desc)
        }
    }
}
