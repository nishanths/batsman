package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

// Flags represents command-line flags.
type Flags struct {
	Http    string
	Watch   bool
	WorkDir string

	Help    bool
	Version bool
}

const perm = 0644
const versionString = `v.0.1.0`
const usageString = `usage: styx [flags...] command [args...]`
const helpString = usageString + `

flags:
  -http     http address to serve site (default: ":8080")
  -watch    rebuild on change, used with serve (default: "false")
  -workdir  path to site's root directory (default: ".")

commands:
  build    build site into "build" directory
  clean    remove "build" directory
  create   create "new post" or "new page"
  help     print this help text
  init     generate scaffolding for new site
  serve    serve "build" directory on localhost
  summary  print site stats
  version  print version`

var (
	flags  = Flags{}
	stdout = log.New(os.Stdout, "", 0)
	stderr = log.New(os.Stderr, "", 0)
)

func main() {
	flag.StringVar(&flags.Http, "http", ":8080", "")
	flag.BoolVar(&flags.Watch, "watch", false, "")
	flag.StringVar(&flags.WorkDir, "workdir", ".", "")
	flag.BoolVar(&flags.Help, "help", false, "")
	flag.BoolVar(&flags.Version, "version", false, "")

	flag.Usage = func() {
		stderr.Println(helpString)
		os.Exit(2)
	}
	flag.Parse()

	if flags.Help {
		stdout.Println(helpString)
		os.Exit(0)
	}
	if flags.Version {
		stdout.Println(versionString)
		os.Exit(0)
	}

	command := flag.Arg(0)
	if command == "" {
		stderr.Println("styx: require command")
		stderr.Println(usageString)
		stderr.Println("see 'styx help' for more details")
		os.Exit(2)
	}

	switch command {
	case "help":
		stdout.Println(helpString)
		os.Exit(0)
	case "version":
		stdout.Println(versionString)
		os.Exit(0)
	}

	if err := computeWorkDir(); err != nil {
		stderr.Println(err)
		os.Exit(1)
	}

	switch command {
	case "build":
	case "clean":
		do(clean)
	case "create":
	case "init":
		do(initialize)
	case "serve":
	case "summary":
	default:
		stderr.Printf("styx: unknown command %q\n", command)
		stderr.Println("see 'styx help' for more details")
		os.Exit(2)
	}
}

func computeWorkDir() error {
	flags.WorkDir = path.Clean(flags.WorkDir)

	if !path.IsAbs(flags.WorkDir) {
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("styx: failed to determine workdir: %s", err)
		}
		flags.WorkDir = path.Join(wd, flags.WorkDir)
	}

	info, err := os.Stat(flags.WorkDir)
	if err != nil {
		return fmt.Errorf("styx: failed to determine workdir: %s", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("styx: workdir %q should be directory", flags.WorkDir)
	}

	return nil
}

type CmdFunc func(args ...string) error

// do executes the supplied command func with flag.Args
// and returns with a non-zero exit code
// if the returned error is non-nil,
// and with 0 if the error is nil.
func do(fn CmdFunc) {
	if err := fn(flag.Args()...); err != nil {
		stderr.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

// isEmpty returns whether a directory is empty.
func isEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases.
}

func clean(_ ...string) error {
	return os.RemoveAll(path.Join(flags.WorkDir, "build"))
}

func initialize(args ...string) error {
	if len(args) < 2 || args[1] == "" {
		return errors.New("styx: init requires path argument\nexample: styx init myblog")
	}

	if err := os.MkdirAll(flags.WorkDir, perm); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("styx: path %q already exists")
		}
		return fmt.Errorf("styx: %s", err)
	}
	for _, p := range []string{"posts", "pages", "css", "js"} {
		if err := os.Mkdir(path.Join(flags.WorkDir, p), perm); err != nil {
			return fmt.Errorf("styx: %s", err)
		}
	}
	return nil
}
