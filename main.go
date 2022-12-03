package main

import (
	"fmt"
	"os/exec"
	"strings"
)

type IShellCommand interface {
	SetDir(string)
	Output() ([]byte, error)
	Wait() error
}

type execShellCommand struct {
	*exec.Cmd
}

func (exc execShellCommand) SetDir(dir string) {
	exc.Dir = dir
}

func newExecShellCommander(name string, arg ...string) IShellCommand {
	execCmd := exec.Command(name, arg...)
	return execShellCommand{Cmd: execCmd}
}

// override this in tests to mock the git shell command
var shellCommander = newExecShellCommander

func myFuncThatUsesExecCmd() {
	cmd := shellCommander("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.SetDir("mydir")
	output, err := cmd.Output()
	if err != nil {
		fmt.Println("Git rev-parse failed")
		return
	}

	gitCurrentBranch := strings.TrimSpace(string(output))
	fmt.Printf("Git branch is '%v'\n", gitCurrentBranch)
}
