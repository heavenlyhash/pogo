// Copyright 2013 Eric Myhre
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gosh

import (
	"bytes"
	"fmt"
	"os/exec"
	"polydawn.net/pogo/iox"
)

func Sh(cmd string) Command {
	var cmdt commandTemplate
	cmdt.cmd = cmd
	cmdt.env = getOsEnv()
	cmdt.OkExit = []int{0}
	return enclose(&cmdt)
}

type Command func(args ...interface{}) Command

// private type, used exactly once to create a const nobody else can create so we can use it as a flag to trigger private behavior
type expose_t bool

const expose expose_t = true

type exposer struct{ cmdt *commandTemplate }

func closure(cmdt commandTemplate, args ...interface{}) Command {
	if len(args) == 0 {
		// an empty call is a synonym for Command.Run().
		// if you want to just get a RunningCommand reference to track, use Command.Start() instead.
		enclose(&cmdt).Run()
		return nil
	} else if args[0] == expose {
		// produce a function that when called with an exposer, exposes its cmdt.
		return func(x ...interface{}) Command {
			t := x[0].(*exposer)
			t.cmdt = &cmdt
			return nil
		}
	} else {
		// examine each of the arguments, modify our (already forked) cmdt, and
		//  return a new callable Command closure with the newly baked command template.
		for _, rarg := range args {
			switch arg := rarg.(type) {
			case string:
				cmdt.bakeArgs(arg)
			case Env:
				cmdt.bakeEnv(arg)
			case ClearEnv:
				cmdt.clearEnv()
			case Opts:
				cmdt.bakeOpts(arg)
			default:
				panic(IncomprehensibleCommandModifier{wat: &rarg})
			}
		}
		return enclose(&cmdt)
	}
}

func (f Command) expose() *commandTemplate {
	var t exposer
	f(expose)(&t)
	return t.cmdt
}

func enclose(cmdt *commandTemplate) Command {
	return func(x ...interface{}) Command {
		return closure(*cmdt, x...)
	}
}

func (f Command) BakeArgs(args ...string) Command {
	return enclose(f.expose().bakeArgs(args...))
}

func (cmdt *commandTemplate) bakeArgs(args ...string) *commandTemplate {
	cmdt.args = append(cmdt.args, args...)
	return cmdt
}

func (f Command) BakeEnv(args Env) Command {
	return enclose(f.expose().bakeEnv(args))
}

func (cmdt *commandTemplate) bakeEnv(args Env) *commandTemplate {
	for k, v := range args {
		if v == "" {
			delete(cmdt.env, k)
		} else {
			cmdt.env[k] = v
		}
	}
	return cmdt
}

func (f Command) ClearEnv() Command {
	return enclose(f.expose().clearEnv())
}

func (cmdt *commandTemplate) clearEnv() *commandTemplate {
	cmdt.env = make(map[string]string)
	return cmdt
}

func (f Command) BakeOpts(args ...Opts) Command {
	return enclose(f.expose().bakeOpts(args...))
}

func (cmdt *commandTemplate) bakeOpts(args ...Opts) *commandTemplate {
	for _, arg := range args {
		if arg.Cwd != "" {
			cmdt.Cwd = arg.Cwd
		}
		if arg.In != nil {
			cmdt.In = arg.In
		}
		if arg.Out != nil {
			cmdt.Out = arg.Out
		}
		if arg.Err != nil {
			cmdt.Err = arg.Err
		}
		if arg.OkExit != nil {
			cmdt.OkExit = arg.OkExit
		}
	}
	return cmdt
}

/**
 * Starts execution of the command.  Returns a reference to a RunningCommand,
 * which can be used to track execution of the command, configure exit listeners,
 * etc.
 */
func (f Command) Start() *RunningCommand {
	cmdt := f.expose()
	rcmd := exec.Command(cmdt.cmd, cmdt.args...)

	// set up env
	if cmdt.env != nil {
		rcmd.Env = make([]string, len(cmdt.env))
		i := 0
		for k, v := range cmdt.env {
			rcmd.Env[i] = fmt.Sprintf("%s=%s", k, v)
			i++
		}
	}

	// set up opts (cwd/stdin/stdout/stderr)
	if cmdt.Cwd != "" {
		rcmd.Dir = cmdt.Cwd
	}
	if cmdt.In != nil {
		switch in := cmdt.In.(type) {
		case Command:
			//TODO something marvelous
			panic(fmt.Errorf("not yet implemented"))
		default:
			rcmd.Stdin = iox.ReaderFromInterface(in)
		}
	}
	if cmdt.Out != nil {
		rcmd.Stdout = iox.WriterFromInterface(cmdt.Out)
	}
	if cmdt.Err != nil {
		if cmdt.Err == cmdt.Out {
			rcmd.Stderr = rcmd.Stdout
		} else {
			rcmd.Stderr = iox.WriterFromInterface(cmdt.Err)
		}
	}

	// go time
	cmd := NewRunningCommand(rcmd)
	cmd.Start()
	return cmd
}

/**
 * Starts execution of the command, and waits until completion before returning.
 * If the command does not execute successfully, a panic of type FailureExitCode
 * will be emitted; use Opts.OkExit to configure what is considered success.
 *
 * The is exactly the behavior of a no-arg invokation on an Command, i.e.
 *   `Sh("echo")()`
 * and
 *   `Sh("echo").Run()`
 * are interchangable and behave identically.
 *
 * Use the Start() method instead if you need to run a task in the background, or
 * if you otherwise need greater control over execution.
 */
func (f Command) Run() {
	cmdt := f.expose()
	cmd := f.Start()
	cmd.Wait()
	exitCode := cmd.GetExitCode()
	for _, okcode := range cmdt.OkExit {
		if exitCode == okcode {
			return
		}
	}
	panic(FailureExitCode{cmdname: cmdt.cmd, code: exitCode})
}

/**
 * Starts execution of the command, waits until completion, and then returns the
 * accumulated output of the command as a string.  As with Run(), a panic will be
 * emitted if the command does not execute successfully.
 *
 * This does not include output from stderr; use CombinedOutput() for that.
 *
 * This acts as BakeOpts() with a value set on the Out field; that is, it will
 * overrule any previously configured output, and also it has no effect on where
 * stderr will go.
 */
func (f Command) Output() string {
	var buf bytes.Buffer
	f.BakeOpts(Opts{Out: &buf}).Run()
	return buf.String()
}

/**
 * Same as Output(), but acts on both stdout and stderr.
 */
func (f Command) CombinedOutput() string {
	var buf bytes.Buffer
	f.BakeOpts(Opts{Out: &buf, Err: &buf}).Run()
	return buf.String()
}
