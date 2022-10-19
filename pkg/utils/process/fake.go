package process

import (
	"strings"
)

// An instance of a ProcessExecutor that keeps the history of commands
// for inspection and returns the predefined results.
// Even when it allows multiple invocations to Exec, it only allows
// setting one err and output which are returned on each call. If different
// results are needed for each invocation, [CallbackProcessExecutor] may a
// better alternative
type FakeProcessExecutor struct {
	invocations int
	commands    []string
	err         error
	output      []byte
}

// NewFakeProcessExecutor creates a new instance of a ProcessExecutor
func NewFakeProcessExecutor(output []byte, err error) *FakeProcessExecutor {
	return &FakeProcessExecutor{
		err:    err,
		output: output,
	}
}

func (p *FakeProcessExecutor) updateHistory(cmd string, args ...string) {
	cmdLine := cmd + " " + strings.Join(args, " ")
	p.commands = append(p.commands, cmdLine)
	p.invocations++
}

// Exec mocks the executing of the process according to
func (p *FakeProcessExecutor) Exec(cmd string, args ...string) ([]byte, error) {
	p.updateHistory(cmd, args...)
	return p.output, p.err
}

// Invoked indicates if the Exec command was invoked at least once
func (p *FakeProcessExecutor) Invoked() bool {
	return p.invocations > 0
}

// Cmd returns the value of the last command passed to the last invocation
func (p *FakeProcessExecutor) Cmd() string {
	if p.invocations == 0 {
		return ""
	}
	return p.commands[p.invocations-1]
}

// CmdHistory returns the history of commands executed. If Invocations is 0, returns
// an empty array
func (p *FakeProcessExecutor) CmdHistory() []string {
	return p.commands
}

// Invocations returns the number of invocations to the Exec function
func (p *FakeProcessExecutor) Invocations() int {
	return p.invocations
}

// Reset clears the history of invocations to the FakeProcessExecutor
func (p *FakeProcessExecutor) Reset() {
	p.invocations = 0
	p.commands = []string{}
}

// ExecCallback defines a function that can receive the forward of an Exec invocation
// The function must return the output of the invocation and the execution error, if any
type ExecCallback func(cmd string, args ...string) ([]byte, error)

// A fake process Executor that forwards the invocations of the exec to
// a function that can dynamically return error and output.
type CallbackProcessExecutor struct {
	FakeProcessExecutor
	callback ExecCallback
}

// Forward exec to the callback
func (c *CallbackProcessExecutor) Exec(cmd string, args ...string) ([]byte, error) {
	// update command history but ignore outputs
	c.FakeProcessExecutor.updateHistory(cmd, args...)
	// return outputs from callback
	return c.callback(cmd, args...)
}

func NewCallbackProcessExecutor(callback ExecCallback) *CallbackProcessExecutor {
	return &CallbackProcessExecutor{
		callback: callback,
	}
}
