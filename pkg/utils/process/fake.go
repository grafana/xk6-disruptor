package process

import (
	"strings"
)

// FakeExecutor is an instance of a ProcessExecutor that keeps the history
// of commands for inspection and returns the predefined results.
// Even when it allows multiple invocations to Exec, it only allows
// setting one err and output which are returned on each call. If different
// results are needed for each invocation, [CallbackExecutor] may a
// better alternative
type FakeExecutor struct {
	invocations int
	commands    []string
	err         error
	output      []byte
}

// NewFakeExecutor creates a new instance of a ProcessExecutor
func NewFakeExecutor(output []byte, err error) *FakeExecutor {
	return &FakeExecutor{
		err:    err,
		output: output,
	}
}

func (p *FakeExecutor) updateHistory(cmd string, args ...string) {
	cmdLine := cmd + " " + strings.Join(args, " ")
	p.commands = append(p.commands, cmdLine)
	p.invocations++
}

// Exec mocks the executing of the process according to
func (p *FakeExecutor) Exec(cmd string, args ...string) ([]byte, error) {
	p.updateHistory(cmd, args...)
	return p.output, p.err
}

// Invoked indicates if the Exec command was invoked at least once
func (p *FakeExecutor) Invoked() bool {
	return p.invocations > 0
}

// Cmd returns the value of the last command passed to the last invocation
func (p *FakeExecutor) Cmd() string {
	if p.invocations == 0 {
		return ""
	}
	return p.commands[p.invocations-1]
}

// CmdHistory returns the history of commands executed. If Invocations is 0, returns
// an empty array
func (p *FakeExecutor) CmdHistory() []string {
	return p.commands
}

// Invocations returns the number of invocations to the Exec function
func (p *FakeExecutor) Invocations() int {
	return p.invocations
}

// Reset clears the history of invocations to the FakeProcessExecutor
func (p *FakeExecutor) Reset() {
	p.invocations = 0
	p.commands = []string{}
}

// ExecCallback defines a function that can receive the forward of an Exec invocation
// The function must return the output of the invocation and the execution error, if any
type ExecCallback func(cmd string, args ...string) ([]byte, error)

// CallbackExecutor is fake process Executor that forwards the invocations
// to a function that can dynamically return error and output.
type CallbackExecutor struct {
	FakeExecutor
	callback ExecCallback
}

// Exec forwards invocation to the callback
func (c *CallbackExecutor) Exec(cmd string, args ...string) ([]byte, error) {
	// update command history but ignore outputs
	c.FakeExecutor.updateHistory(cmd, args...)
	// return outputs from callback
	return c.callback(cmd, args...)
}

// NewCallbackExecutor returns an instance of a CallbackExecutor
func NewCallbackExecutor(callback ExecCallback) *CallbackExecutor {
	return &CallbackExecutor{
		callback: callback,
	}
}
