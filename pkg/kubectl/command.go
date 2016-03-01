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
