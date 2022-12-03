# go-test-mock-exec-command

One-line summary: How to code golang tests for code that uses `exec.Command()`

Community feedback and/or contributions are welcome: open an issue or a PR, or start a 
discussion in github repo.

## Background 

Testing in golang can take a bit of getting used to. Especially for unit tests, where you should
decouple your tests from systems external to the application.

Eg, if your app interacts with the local filesystem (reads and/or writes files), do you really want
your test to create and remove temporary files every time the test runs? If it connects to a
database in a cloud provider, do you really want your test to create the DB, initialize a schema,
seed it with data, etc, at the beginning of each test run?

For integration testing, the answer is likely yes, but for unit testing, you should aim
for testing the unit, not its interactions with complex components that are external to your
application. Other examples of external components are cloud API (AWS, GCP, Azure, etc) and
kubernetes.

This repo aims to demonstrate how to design application code that uses the `os/exec` (such 
as `exec.Command()` and `exec.Cmd`) so that it can be mocked by your golang test, WITHOUT 
resorting the environment-variable based re-run of `go test` by `go test` seen in many blogs
on the web. 

The same approach can be applied to all situations mentioned above (filesystem, shell, cloud, etc).

## The Problem

Say your golang application has the following code that uses `exec`:
```go
package foo

import "os/exec"

func funcThatUsesExecCmd() {
    cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
    cmd.Dir = "mydir"
    output, err := cmd.Output()
    if err != nil {
        // handle error
    } else {
        // process & handle output
    }
}
```

Let's say for simplicity that this code is in `foo.go` in the `foo` folder. So you create
`foo_test.go` in the same folder:
```go
package foo

import "testing"

func Test_myFuncThatUsesExecCmd(t *testing.T) {
    // setup use case 1
    funcThatUsesExecCmd()
    // setup use case 2
    funcThatUsesExecCmd()
    // setup use case 3
    funcThatUsesExecCmd()
    // setup use case 4
    funcThatUsesExecCmd()
}
```

Whenever you run `go test`, this will call `git rev-parse` from `mydir`. For this to work, your
test would have to create `mydir`, install git, git init `mydir`, and eventually cleanup.
You could create a docker image that has exactly what the test needs, but this will be hard to
maintain for unit testing, where you want to test many different conditions.

It would be much better to just "replace" running "git" by what our application uses internally,
namely the git command's output. Keeping in mind that the solution that does this to have
minimal impact on your code.

The main technique mentioned in blogs and posts on the web is the one used by the authors of
the `os/exec` module itself: use an environment variable to select behavior to be run in a re-run of
your test by `go test` using that behavior. If you find that hard to understand, you're not alone. I
am sure the authors had a very good reason to use that approach, but I am certain that they would
NOT recommend it for an application or module written in modern go. Indeed writing tests for a
low-level library like `os/exec`, that is part of the language's standard library, is subject to
very different constraints from testing your own app. Moreover,

- the technique does not scale well at the application level: you'll end up with as many behaviors
  as you have tests that involve the shell exec, and each one will be re-running go test with a
  modified environment variable value!
- Go has all the tools necessary to do this much more understandably, using Go's excellent take on
  polymorphism, sand in a way that applies to other test situations.

## The Solution

The solution that I'm going to discuss here is not new by any means; I have seen it mentioned
in the context of other go test questions / hurdles, and it's been used in C++ and Python
since the dawn of those languages.

The design is fairly simple:

### Application side:

In your `foo.go`,

1. Determine what I/O functions/methods need to be replaced: print, read/write file, shell exec, AWS
   query, kubernetes API server request.
2. Create an interface for the portion of the API that our app uses
3. Create a package-level var that points to a struct that implements that interface
4. Do a few modifications to your application code so it uses this package-level var, instead of
   directly using `exec`

### Test side:

In your `foo_test.go`,

1. define a new struct that implements the interface created
2. make the test replace the package-level var: make it point to an instance of your struct,
   configured to represent the net effect of your calls to `exec`
3. run your test

## Example

It is much easier to understand with an example, and this is what this git repo is for.

### Application side:

In `foo.go`:

1. In the above example, the API we need to replace is
   creation of the `exec.Cmd` object, setting `Dir` on it, and calling its `Output()` method.
   A bigger application might have other functions that use more of the `os/exec` API, and these
   methods would have to be included.
2. Create interface:
    ```go
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
    ```
3. Create package-level var:
    ```go
    func newExecShellCommander(name string, arg ...string) IShellCommand {
        execCmd := exec.Command(name, arg...)
        return execShellCommand{Cmd: execCmd}
    }
    
    // override this in tests to mock the git shell command
    var shellCommander = newExecShellCommander
    ```
4. Adjust application code to use the package var:
    ```go
    func myFuncThatUsesExecCmd() {
        cmd := shellCommander("git", "rev-parse", "--abbrev-ref", "HEAD")
        cmd.SetDir("mydir")
        output, err := cmd.Output()
        if err != nil {
            // handle error
        } else {
            // process & handle output
        }
    }
    ```

Note however that the code from steps 2 and 3 need not be in `foo.go` if there are other places
in your application that use `os/exec`. In that case step 1 might identify a few more methods,
and steps 2 and 3 would be in a package used by `foo` and other places in your application, and
the `execShellCommand` of step 2 might have to implement more methods (probably only if
attribrutes other than `Dir` are used).

### Test side:

1. Define a new struct that implements the interface created
    ```go
    type myShellCommand struct {
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
    ```
2. Make the test replace the package-level var: make it point to an instance of your struct,
   configured to represent the net effect of your calls to `exec`
    ```go
    func Test_myFuncThatUsesExecCmd(t *testing.T) {
        // temporarily swap the shell commander
        curShellCommander := shellCommander
        defer func() { shellCommander = curShellCommander }()
        shellCommander = func(name string, arg ...string) IShellCommand {
            fmt.Printf("exec.Command() for %v called with %v and %v\n", testName, name, arg)
            return myShellCommand{
                OutputterFunc: func() ([]byte, error) {
                    fmt.Printf("Output obtained for %v\n", testName),
                }
            }
        }
   
        myFuncThatUsesExecCmd()
      }
   ```
3. Run your test

You will notice that as you add test cases, the `shellCommander = func...` block of code will
be repeated many times with very little modification. You can refactor this code into a
function that creates a shellCommander function that returns the desired output OR error:

```go
type execCommandFunc func (name string, arg ...string) IShellCommand

func newMockShellCommanderForOutput(output string, err error) execCommandFunc {
    return func (name string, arg ...string) IShellCommand {
        fmt.Printf("exec.Command() called with %v and %v\n", name, arg)
        outputterFunc := func () ([]byte, error) {
            if err == nil {
                fmt.Println("Output obtained")
            } else {
                fmt.Println("Failed to get Output")
            }
            return []byte(output), err
        }
        return myShellCommand{
            OutputterFunc: outputterFunc,
        }
    }
}
```

The `execCommandFunc` simplifies the signature of the refactored function.

With the above, you can now write your test like this:

```go
func Test_myFuncThatUsesExecCmd(t *testing.T) {
// temporarily swap the shell commander
curShellCommander := shellCommander
defer func () { shellCommander = curShellCommander }()

// happy path: 
shellCommander = newMockShellCommanderForOutput("hello", nil)
myFuncThatUsesExecCmd()
// check things

// sad path: 
shellCommander = newMockShellCommanderForOutput("nil", errors.New("some error"))
myFuncThatUsesExecCmd()
// check things
}
```

The actual code in this repo additionally passes `t` to the generator so the test name
can be used in the output (and other operations on `t` might be useful). Here is the output
on my system:

```text
exec.Command() for Test_myFuncThatUsesExecCmd called with git and [rev-parse --abbrev-ref HEAD]
Output obtained for Test_myFuncThatUsesExecCmd
Git branch is 'hello'
exec.Command() for Test_myFuncThatUsesExecCmd called with git and [rev-parse --abbrev-ref HEAD]
Failed to get Output for Test_myFuncThatUsesExecCmd
Git rev-parse failed
PASS
ok      mock_exec    0.003s
```

The test code will also be clearer if it uses table-based test-cases with `t.Run()`, but this goes
beyond what is needed for this discussion.

### Run the code in this repo

1. install go
2. run `go test`

## Conclusion

There is no need to subvert the go test system with environment variables as done in the approach
commonly recommended for mocking `os/exec` usage. Simply create a wrapper interface and a default
wrapper instance on the application side, and make the test override the default wrapper with a
custom one that encapsulates the next effect of running the `exec.Cmd`. The test code can be
refactored so that table-based testing can be done easily. The approach works for any I/O that needs
to be mocked, whether it is the filesystem, a database, a cloud provider, etc. 