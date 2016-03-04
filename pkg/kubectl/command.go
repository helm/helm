/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package kubectl

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

type cmd struct {
	*exec.Cmd
}

func command(args ...string) *cmd {
	return &cmd{exec.Command(Path, args...)}
}

func assignStdin(cmd *cmd, in []byte) {
	fmt.Println(string(in))
	cmd.Stdin = bytes.NewBuffer(in)
}

func (c *cmd) String() string {
	var stdin string

	if c.Stdin != nil {
		b, _ := ioutil.ReadAll(c.Stdin)
		stdin = fmt.Sprintf("< %s", string(b))
	}

	return fmt.Sprintf("[CMD] %s %s", strings.Join(c.Args, " "), stdin)
}
