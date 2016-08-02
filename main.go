package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

const versionString = "0.1.0"
const usageString = `usage: styx [-exclude=<f1,f2>] [-http=<addr>] [-watch] [-workdir=<dir>] <command>`
const helpString = usageString + `

flags:
  -exclude  comma-separated list of filename patterns to exclude in build (default: "")
  -http     http address to serve site (default: ":8080")
  -watch    whether to rebuild on changes while serving (default: "false")
  -workdir  path to site's root directory (default: ".")

commands:
  build    generate site into "build" directory
  create   create markdown file at specified path
  init     initalize new site at specified path
  serve    serve "build" directory
  summary  print site summary`

var (
	perm = struct {
		file, dir os.FileMode
	}{0644, 0755}

	flags = struct {
		Exclude []string
		Http    string
		Watch   bool
		WorkDir string
		Help    bool
		Version bool
	}{}

	stdout = log.New(os.Stdout, "", 0)
	stderr = log.New(os.Stderr, "", 0)
)

func main() {
	var excl string
	flag.StringVar(&excl, "exclude", "", "")
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
	flags.Exclude = strings.Split(excl, ",")

	if flags.Help {
		stdout.Println(helpString)
		os.Exit(0)
	}
	if flags.Version {
		stdout.Println("v" + versionString)
		os.Exit(0)
	}

	command := flag.Arg(0)
	if command == "" {
		stderr.Println("styx: require command")
		stderr.Println(usageString)
		stderr.Println(`run "styx -help" for more details`)
		os.Exit(2)
	}

	switch command {
	case "help":
		stdout.Println(helpString)
		os.Exit(0)
	case "version":
		stdout.Println("v" + versionString)
		os.Exit(0)
	}

	if err := computeWorkDir(); err != nil {
		stderr.Println(err)
		os.Exit(1)
	}

	switch command {
	case "build":
		do(build)
	case "create":
	case "init":
		do(initialize)
	case "serve":
		do(serve)
	case "summary":
	default:
		stderr.Printf("styx: unknown command %q\n", command)
		stderr.Println(`run "styx -help" for more details`)
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

type CmdFunc func(args ...string) error

// do executes the supplied CmdFunc with flag.Args
// and exits with a non-zero exit code
// if the returned error is non-nil,
// and with 0 if the error is nil.
func do(fn CmdFunc) {
	if err := fn(flag.Args()...); err != nil {
		stderr.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

type WrapError struct {
	Err error
}

func (e WrapError) Error() string {
	return fmt.Sprintf("styx: %s", e.Err.Error())
}

func initialize(args ...string) error {
	if len(args) < 2 || args[1] == "" {
		return errors.New("styx: init requires path argument\nexample: styx init myblog")
	}

	root := path.Join(flags.WorkDir, args[1])
	success := false

	defer func() {
		if !success {
			os.RemoveAll(root) // Ignore error.
		}
	}()

	// Root path

	if err := os.MkdirAll(path.Join(root, "src"), perm.dir); err != nil {
		// TODO: Test this.
		if os.IsExist(err) {
			return fmt.Errorf("styx: path %q already exists")
		}
		return WrapError{err}
	}

	// Files

	wg := sync.WaitGroup{}
	errs := make(chan error, len(rawFiles))
	for k, v := range rawFiles {
		k, v := k, v
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- createFile(
				filepath.Join(root, filepath.FromSlash(k)),
				bytes.NewReader(v),
			)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return WrapError{err}
		}
	}

	success = true
	return nil
}

func createFile(name string, data io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(name), perm.dir); err != nil {
		return err
	}
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm.file)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, data)
	return err
}
