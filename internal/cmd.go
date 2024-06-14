package cli

import (
	"io"
	"os/exec"
)

func prepareCmd(stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stdin = stdin
	cmd.Stderr = stderr

	return cmd
}
