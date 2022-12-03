package main

import (
	"errors"
	"fmt"
	"testing"
)

type myShellCommand struct {
	//IShellCommand
	OutputterFunc func() ([]byte, error)
	WaiterFunc    func() error
}

func (sc myShellCommand) Output() ([]byte, error) {
	return sc.OutputterFunc()
}

func (sc myShellCommand) SetDir(_ string) {}

func (sc myShellCommand) Wait() error {
	return sc.WaiterFunc()
}

type execCommandFunc func(name string, arg ...string) IShellCommand

func newMockShellCommanderForOutput(output string, err error, t *testing.T) execCommandFunc {
	testName := t.Name()
	return func(name string, arg ...string) IShellCommand {
		fmt.Printf("exec.Command() for %v called with %v and %v\n", testName, name, arg)
		outputterFunc := func() ([]byte, error) {
			if err == nil {
				fmt.Printf("Output obtained for %v\n", testName)
			} else {
				fmt.Printf("Failed to get Output for %v\n", testName)
			}
			return []byte(output), err
		}
		return myShellCommand{
			OutputterFunc: outputterFunc,
		}
	}
}

func Test_myFuncThatUsesExecCmd(t *testing.T) {
	// temporarily swap the shell commander
	curShellCommander := shellCommander
	defer func() { shellCommander = curShellCommander }()

	shellCommander = newMockShellCommanderForOutput("hello", nil, t)
	myFuncThatUsesExecCmd()

	shellCommander = newMockShellCommanderForOutput("nil", errors.New("some error"), t)
	myFuncThatUsesExecCmd()
}
