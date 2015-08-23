package main

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/constabulary/gb"
	"github.com/constabulary/gb/cmd"
)

var (
	fs   = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cwd  string
	args []string
)

const (
	noDestroyContext = false
)

func init() {
	fs.BoolVar(&gb.Quiet, "q", gb.Quiet, "suppress log messages below ERROR level")
	fs.BoolVar(&gb.Verbose, "v", gb.Verbose, "enable log levels below INFO level")
	fs.StringVar(&cwd, "R", cmd.MustGetwd(), "set the project root") // actually the working directory to start the project root search

	fs.Usage = usage
}

var commands = make(map[string]*cmd.Command)

// registerCommand registers a command for main.
// registerCommand should only be called from init().
func registerCommand(command *cmd.Command) {
	commands[command.Name] = command
}

func main() {
	args := os.Args
	if len(args) < 2 || args[1] == "-h" {
		fs.Usage()
		os.Exit(1)
	}
	name := args[1]
	if name == "help" {
		help(args[2:])
		return
	}

	verifyGoVersion()

	command, ok := commands[name]
	if (command != nil && !command.Runnable()) || !ok {
		if _, err := lookupPlugin(name); err != nil {
			gb.Errorf("unknown command %q", name)
			fs.Usage()
			os.Exit(1)
		}
		command = commands["plugin"]
	}

	// add extra flags if necessary
	if command.AddFlags != nil {
		command.AddFlags(fs)
	}

	var err error
	if command.FlagParse != nil {
		err = command.FlagParse(fs, args)
	} else {
		err = fs.Parse(args[2:])
	}
	if err != nil {
		gb.Fatalf("could not parse flags: %v", err)
	}

	args = fs.Args() // reset args to the leftovers from fs.Parse
	if command == commands["plugin"] {
		args = append([]string{name}, args...)
	}
	cwd, err := filepath.Abs(cwd) // if cwd was passed in via -R, make sure it is absolute
	if err != nil {
		gb.Fatalf("could not make project root absolute: %v", err)
	}

	ctx, err := cmd.NewContext(
		cwd, // project root
		gb.GcToolchain(),
		gb.Gcflags(gcflags),
		gb.Ldflags(ldflags),
	)
	if err != nil {
		gb.Fatalf("unable to construct context: %v", err)
	}

	if !noDestroyContext {
		defer ctx.Destroy()
	}

	if command.ParseArgs != nil {
		args = command.ParseArgs(ctx, ctx.Projectdir(), args)
	} else {
		args = cmd.ImportPaths(ctx, cwd, args)
	}

	gb.Debugf("args: %v", args)
	if err := command.Run(ctx, args); err != nil {
		gb.Fatalf("command %q failed: %v", name, err)
	}
}

// verify that the version of Go that compiled this binary is still correct.
func verifyGoVersion() {
	want := runtime.Version()
	filename := filepath.Join(runtime.GOROOT(), "VERSION")
	if strings.Contains(want, "devel") {
		filename += ".cache"
	}
	got, err := ioutil.ReadFile(filename)
	if err != nil {
		gb.Fatalf("cannot validate Go version: %v", err)
	}
	if want != string(got) {
		gb.Fatalf("Go version does not match: gb was compiled with %q, installed Go version from %v is %q", want, runtime.GOROOT(), got)
	}
}
