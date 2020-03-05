/*
Copyright (c) for portions of walk_test.go are held by The Go Authors, 2009 and are
provided under the BSD license.

https://github.com/golang/go/blob/master/LICENSE

Copyright The Helm Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sympath

import (
	"os"
	"path/filepath"
	"testing"
)

type Node struct {
	name          string
	entries       []*Node // nil if the entry is a file
	marks         int
	expectedMarks int
	symLinkedTo   string
}

var tree = &Node{
	"testdata",
	[]*Node{
		{"a", nil, 0, 1, ""},
		{"b", []*Node{}, 0, 1, ""},
		{"c", nil, 0, 2, ""},
		{"d", nil, 0, 0, "c"},
		{
			"e",
			[]*Node{
				{"x", nil, 0, 1, ""},
				{"y", []*Node{}, 0, 1, ""},
				{
					"z",
					[]*Node{
						{"u", nil, 0, 1, ""},
						{"v", nil, 0, 1, ""},
						{"w", nil, 0, 1, ""},
					},
					0,
					1,
					"",
				},
			},
			0,
			1,
			"",
		},
	},
	0,
	1,
	"",
}

func walkTree(n *Node, path string, f func(path string, n *Node)) {
	f(path, n)
	for _, e := range n.entries {
		walkTree(e, filepath.Join(path, e.name), f)
	}
}

func makeTree(t *testing.T) {
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.entries == nil {
			if n.symLinkedTo != "" {
				if err := os.Symlink(n.symLinkedTo, path); err != nil {
					t.Fatalf("makeTree: %v", err)
				}
			} else {
				fd, err := os.Create(path)
				if err != nil {
					t.Fatalf("makeTree: %v", err)
					return
				}
				fd.Close()
			}
		} else {
			if err := os.Mkdir(path, 0770); err != nil {
				t.Fatalf("makeTree: %v", err)
			}
		}
	})
}

func checkMarks(t *testing.T, report bool) {
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.marks != n.expectedMarks && report {
			t.Errorf("node %s mark = %d; expected %d", path, n.marks, n.expectedMarks)
		}
		n.marks = 0
	})
}

// Assumes that each node name is unique. Good enough for a test.
// If clear is true, any incoming error is cleared before return. The errors
// are always accumulated, though.
func mark(info os.FileInfo, err error, errors *[]error, clear bool) error {
	if err != nil {
		*errors = append(*errors, err)
		if clear {
			return nil
		}
		return err
	}
	name := info.Name()
	walkTree(tree, tree.name, func(path string, n *Node) {
		if n.name == name {
			n.marks++
		}
	})
	return nil
}

func TestWalk(t *testing.T) {
	makeTree(t)
	errors := make([]error, 0, 10)
	clear := true
	markFn := func(path string, info os.FileInfo, err error) error {
		return mark(info, err, &errors, clear)
	}
	// Expect no errors.
	err := Walk(tree.name, markFn)
	if err != nil {
		t.Fatalf("no error expected, found: %s", err)
	}
	if len(errors) != 0 {
		t.Fatalf("unexpected errors: %s", errors)
	}
	checkMarks(t, true)

	// cleanup
	if err := os.RemoveAll(tree.name); err != nil {
		t.Errorf("removeTree: %v", err)
	}
}
