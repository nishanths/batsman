package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/russross/blackfriday"
)

const versionString = "0.1.0"
const usageString = `usage: styx [flags]... <command>`
const helpString = usageString + `

flags:
  -http        http address to serve site (default: "localhost:8080")
  -watch       whether to regenerate static files on change while serving (default: false)
  -title       title of new markdown file (default: "")
  -draft       whether new markdown file is draft (default: false)
  -workdir     path to site's root directory (default: "./")

commands:
  init         initialize new site at specified path
  new          prints contents of new markdown file to stdout
  build        generate static files into the "build/" directory
  serve        serve the "build/" directory via http
  summary      print site summary to stdout`

var (
	perm = struct {
		file, dir os.FileMode
	}{0644, 0755}

	stdout = log.New(os.Stdout, "", 0)
	stderr = log.New(os.Stderr, "", 0)
)

func main() {
	flags := struct {
		HTTP    string
		Watch   bool
		Title   string
		Draft   bool
		WorkDir string

		Help    bool
		Version bool
	}{}

	flag.StringVar(&flags.HTTP, "http", "localhost:8080", "")
	flag.BoolVar(&flags.Watch, "watch", false, "")
	flag.StringVar(&flags.Title, "title", "", "")
	flag.BoolVar(&flags.Draft, "draft", false, "")
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
		stdout.Println("v" + versionString)
		os.Exit(0)
	}

	command := flag.Arg(0)
	switch command {
	case "":
		stderr.Println("styx: error: require command")
		stderr.Println(helpString)
		os.Exit(2)
	case "help":
		stdout.Println(helpString)
		os.Exit(0)
	case "version":
		stdout.Println("v" + versionString)
		os.Exit(0)
	}

	workdir, err := computeAbsDir(flags.WorkDir)
	if err != nil {
		stderr.Println(err)
		os.Exit(1)
	}
	if err := isDirErr(workdir); err != nil {
		stderr.Println(err)
		os.Exit(1)
	}

	switch command {
	case "init":
		do(&Initialize{
			WorkDir: workdir,
			Path:    flag.Arg(1),
		})
	case "new":
		do(&New{
			WorkDir: workdir,
			Title:   flags.Title,
			Draft:   flags.Draft,
		})
	case "build":
		do(&Build{workdir})
	case "serve":
		do(&Serve{
			WorkDir: workdir,
			Watch:   flags.Watch,
			HTTP:    flags.HTTP,
		})
	case "summary":
		do(&Summary{workdir})
	default:
		stderr.Printf("styx: unknown command %q\n", command)
		stderr.Println(`run "styx -help" for usage`)
		os.Exit(2)
	}
}

// isDirErr returns an error if p is not a directory,
// or if any calls fail in the process.
func isDirErr(p string) error {
	info, err := os.Stat(p)
	if err != nil {
		return WrapError{err}
	}
	if !info.IsDir() {
		return fmt.Errorf("styx: workdir %q should be directory", p)
	}
	return nil
}

// computeAbsDir returns he absolute path of p.
// The error is non-nil if the absolute path could not
// be computed or if p is not a directory.
func computeAbsDir(p string) (string, error) {
	p = filepath.Clean(p)
	if filepath.IsAbs(p) {
		return p, nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("styx: failed to determine workdir: %s", err)
	}
	return filepath.Join(wd, p), nil
}

// do calls cmd.Run and exits with exit code 1 if the
// returned error is non-nil or with exit code 0 if
// the error is nil.
func do(cmd Cmd) {
	if err := cmd.Run(); err != nil {
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
	return false, err // Either nil or error, suits both cases.
}

type Cmd interface {
	Run() error
}

type Build struct {
	WorkDir string // Absolute path of work dir.
}

var markdownExts = map[string]bool{
	".md":       true,
	".markdown": true,
}

type FrontMatter struct {
	Draft bool
	Title string
	Time  time.Time
}

type InvalidFrontMatterError struct {
	Key, Val    string
	CorrectVals []string
}

func (e *InvalidFrontMatterError) Error() string {
	return fmt.Sprintf(
		"styx: error: key %q has invalid value %q\nexpected values/formats are: {%s}",
		e.Key, e.Val, strings.Join(e.CorrectVals, ", "),
	)
}

var currentTimeOnce sync.Once
var currTime time.Time

func currentTime() time.Time {
	currentTimeOnce.Do(func() {
		currTime = time.Now().UTC()
	})
	return currTime
}

var knownTimeFormats = []string{
	"2006-01-02 15:04:05 -07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func (f *FrontMatter) fromMap(m map[string]string) error {
	v := m["draft"]
	if v == "true" {
		f.Draft = true
	} else if v != "" && v != "false" {
		return &InvalidFrontMatterError{"draft", v, []string{"true", "false"}}
	}

	f.Title = m["title"]

	v = m["time"]
	if v == "" {
		f.Time = currentTime()
	} else {
		for i, format := range knownTimeFormats {
			t, err := time.Parse(format, v)
			if err == nil {
				f.Time = t
				break
			}
			if i == len(knownTimeFormats)-1 {
				return &InvalidFrontMatterError{"time", v, knownTimeFormats}
			}
		}
	}

	return nil
}

const YAMLFrontMatterSep = `---`

func ParseFrontMatter(r io.Reader) (fm FrontMatter, exists bool, err error) {
	scanner := bufio.NewScanner(r)
	ok := scanner.Scan()
	if !ok {
		return
	}
	first := scanner.Text()
	if first != YAMLFrontMatterSep {
		return // no front matter.
	}
	exists = true

	m := map[string]string{
		"draft": "",
		"title": "",
		"time":  "",
	}
	sep := `:`

	for scanner.Scan() {
		line := scanner.Text()
		if line == YAMLFrontMatterSep {
			break // end of front matter.
		}

		res := strings.SplitN(line, sep, 2)
		if len(res) != 2 {
			err = fmt.Errorf("styx: error: front matter %q should be in format \"key: val\"", line)
			return
		}
		key, val := strings.TrimSpace(res[0]), strings.TrimSpace(res[1])
		m[key] = val
	}

	err = fm.fromMap(m)
	return
}

type TemplateArgs struct {
	Current *Page   // Current file.
	All     []*Page // All files in the same directory.
}

type Page struct {
	Content string    // HTML content generated from markdown.
	Title   string    // Title from front matter.
	Time    time.Time // Timestamp from front matter.
}

func makeLayoutTemplates(root string) (map[string]*template.Template, error) {
	wg := sync.WaitGroup{}
	type result struct {
		Dir      string
		Template *template.Template
		Err      error
	}
	results := make(chan result)

	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			return nil
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			tmpl, err := template.ParseFiles(filepath.Join(p, "_layout.tmpl"))
			results <- result{p, tmpl, err}
		}()
		return nil
	})

	go func() {
		wg.Wait()
		close(results)
	}()

	ret := make(map[string]*template.Template)
	var err error
	for r := range results {
		if r.Err != nil {
			err = r.Err
		}
		ret[r.Dir] = r.Template
	}
	return ret, err
}

func makeAllPages(root string) (map[string][]*Page, error) {
	type result struct {
		Dir  string
		Page *Page
		Err  error
	}
	wg := sync.WaitGroup{}
	results := make(chan result)

	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		if !markdownExts[filepath.Ext(p)] {
			return nil
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			b, err := ioutil.ReadFile(p)
			if err != nil {
				results <- result{Err: err}
				return
			}

			page := &Page{}

			innerWg := sync.WaitGroup{}
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				page.Content = string(blackfriday.MarkdownCommon(b))
				// TODO(nishanths): apply through plugins.
			}()

			fm, ok, err := ParseFrontMatter(bytes.NewReader(b))
			if err != nil {
				results <- result{Err: err}
				return
			}
			if ok {
				page.Title = fm.Title
				page.Time = fm.Time
			}

			innerWg.Wait()
			results <- result{filepath.Dir(p), page, nil}
		}()

		return nil
	})

	go func() {
		wg.Wait()
		close(results)
	}()

	all := make(map[string][]*Page)
	var err error
	for r := range results {
		if r.Err != nil {
			err = r.Err
		}
		all[r.Dir] = append(all[r.Dir], r.Page)
	}
	return all, err
}

func (b *Build) Run() error {
	src := filepath.Join(b.WorkDir, "src")
	build := filepath.Join(b.WorkDir, "build")

	// TODO(nishanths): pickup from here.
	// run in goroutines.
	makeLayoutTemplates(src)
	makeAllPages(src)

	// TODO(nishanths): this may now be outdated.
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		// Copy over non-markdown files.
		if !info.IsDir() {
			rem, err := filepath.Rel(src, p)
			if err != nil {
				return err
			}
			return copyFile(filepath.Join(build, rem), p)
		}

		return nil
	})
}

func copyFile(dst, src string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return createFile(dst, in)
}

type New struct {
	WorkDir string // Absolute path of work dir.
	Title   string
	Draft   bool
}

func (n *New) Run() error {
	panic("new")
}

type Initialize struct {
	WorkDir string // Absolute path of work dir.
	Path    string // Path (absolute or relative) to initialize new site.
}

func (init *Initialize) Run() error {
	if init.Path == "" {
		return errors.New("styx: error: init requires path argument\nexample: styx init /path/to/new/site")
	}

	root, err := computeAbsDir(init.Path)
	if err != nil {
		return err
	}
	success := false

	ok, err := pathExists(root)
	if err != nil {
		return WrapError{err}
	}
	if ok {
		return fmt.Errorf("styx: error: path %q already exists", root)
	}

	defer func() {
		if !success {
			_ = os.RemoveAll(root) // ignore error.
		}
	}()

	if err := os.MkdirAll(root, perm.dir); err != nil {
		return WrapError{err}
	}
	if err := os.Mkdir(filepath.Join(root, "src"), perm.dir); err != nil {
		return WrapError{err}
	}
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

type Serve struct {
	WorkDir string // Absolute path of work dir.
	HTTP    string
	Watch   bool
}

func (s *Serve) Run() error {
	stderr.Printf("serving HTTP on %s ...\n", s.HTTP)
	return http.ListenAndServe(
		s.HTTP,
		http.FileServer(http.Dir(filepath.Join(s.WorkDir, "build"))),
	)
}

type Summary struct {
	WorkDir string
}

func (s *Summary) Run() error {
	panic("summary")
}

func pathExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
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
	if _, err = io.Copy(f, data); err != nil {
		return err
	}
	return f.Sync()
}

type WrapError struct {
	Err error
}

func (e WrapError) Error() string {
	return fmt.Sprintf("styx: error: %s", e.Err.Error())
}
