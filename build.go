package main

import (
	"bytes"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	texttemplate "text/template"
	"time"

	"github.com/russross/blackfriday"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/css"
	"github.com/tdewolff/minify/html"
	"github.com/tdewolff/minify/js"
	"github.com/tdewolff/minify/svg"
)

type Build struct {
	// Plugins is the list of plugins applied
	// on markdown files.
	Plugins texttemplate.FuncMap
}

// MarkdownExts is the extensions considered to be markdown files.
var MarkdownExts = map[string]bool{
	".md":       true,
	".markdown": true,
}

// TemplateArgs contains the data available to each template.
// Current is only available in "layout.tmpl" files.
type TemplateArgs struct {
	Current *Page              // Current file.
	Dir     []*Page            // Pages in the same directory.
	All     map[string][]*Page // All pages in the tree.
}

// Page represents a markdown file.
type Page struct {
	Content template.HTML // HTML content generated from markdown.
	Title   string        // Title from front matter.
	Time    time.Time     // Timestamp from front matter or file's last modified time.
	Path    string        // HTTP path at which the page lives.
}

// ByTime sorts pages in reverse chronological order.
type ByTime []*Page

func (a ByTime) Len() int           { return len(a) }
func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTime) Less(i, j int) bool { return !a[i].Time.Before(a[j].Time) }

func (b *Build) makePages(root string) (pages map[string]*Page, all map[string][]*Page, err error) {
	mx := sync.Mutex{}
	pages = make(map[string]*Page)
	all = make(map[string][]*Page)

	type result struct {
		Dir  string
		Page *Page
		Err  error
	}
	wg := sync.WaitGroup{}
	results := make(chan result)

	err = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !MarkdownExts[filepath.Ext(p)] {
			return nil
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			contents, err := ioutil.ReadFile(p)
			if err != nil {
				results <- result{Err: err}
				return
			}

			page := &Page{}

			innerWg := sync.WaitGroup{}
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				buf := bytes.Buffer{}
				t, err := texttemplate.New("content").Funcs(b.Plugins).Parse(string(contents))
				if err != nil {
					results <- result{Err: err}
					return
				}
				if err := t.Execute(&buf, nil); err != nil {
					results <- result{Err: err}
					return
				}
				page.Content = template.HTML(blackfriday.MarkdownCommon(stripFrontMatter(buf.Bytes())))
			}()

			fm := FrontMatter{}
			err = fm.Parse(bytes.NewReader(contents))
			if err != nil && err != ErrNoFrontMatter {
				results <- result{Err: err}
				return
			}
			if fm.Draft {
				return
			}
			if err != ErrNoFrontMatter {
				page.Title = fm.Title
				page.Time = fm.Time
			} else {
				page.Title = info.Name()
				page.Time = info.ModTime()
			}

			innerWg.Wait()

			mx.Lock()
			pages[p] = page
			mx.Unlock()

			rel, err := filepath.Rel(filepath.Join(".", "src"), p)
			if err != nil {
				results <- result{Err: err}
				return
			}
			page.Path = "/" + path.Join(filepath.ToSlash(stripExt(rel)))
			results <- result{filepath.Dir(rel), page, nil}
		}()

		return nil
	})

	if err != nil {
		return
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.Err != nil {
			err = r.Err
		}
		all[r.Dir] = append(all[r.Dir], r.Page)
	}
	if err != nil {
		return
	}
	for k := range all {
		sort.Sort(ByTime(all[k]))
	}
	return
}

func stripExt(s string) string {
	return strings.TrimSuffix(s, filepath.Ext(s))
}

// changeExt switches the file extension in s to newExt.
// newExt is expected to start with ".". For example, ".txt".
// If s does not have a file extension, newExt is simply appended to s.
func changeExt(s, newExt string) string {
	return stripExt(s) + newExt
}

type minifyFunc func(m *minify.M, w io.Writer, r io.Reader, params map[string]string) error

// minifyFuncs is a map from file extensions to mime type and minify
// function.
//
// Should be kept in sync with the functions registered to the minifier in
// Run.
//
// TODO(nishanths): make minifier.minifyFunc public in minifier pakcage.
// Then we can simply range over this map and register the functions
// instead.
var minifyFuncs = map[string]struct {
	mime string
	fn   minifyFunc
}{
	".css": {"text/css", css.Minify},
	".js":  {"text/javascript", js.Minify},
	".svg": {"image/svg+xml", svg.Minify},
}

func (b *Build) Run() error {
	src := "src"
	build := "build"

	filePage, dirPages, err := b.makePages(src)
	if err != nil {
		return err
	}

	// dirLayout is a map from directory name to the layout template for the
	// directory.
	dirLayout := struct {
		sync.Mutex
		m map[string]*template.Template
	}{m: make(map[string]*template.Template)}

	mf := minify.New()
	mf.Add("text/html", &html.Minifier{})
	mf.AddFunc("text/css", css.Minify)
	mf.AddFunc("text/javascript", js.Minify)
	mf.AddFunc("image/svg+xml", svg.Minify)

	wg := sync.WaitGroup{}
	errs := make(chan error)
	err = filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, minifiable := minifyFuncs[filepath.Ext(p)]

			switch {
			case info.IsDir() || info.Name() == "layout.tmpl":
				return

			case minifiable:
				in, err := os.Open(p)
				if err != nil {
					errs <- err
					return
				}
				defer in.Close()
				rem, err := filepath.Rel(src, p)
				if err != nil {
					errs <- err
					return
				}
				out, err := createFile(filepath.Join(build, rem))
				if err != nil {
					errs <- err
					return
				}
				defer out.Close()
				if err := minifyFuncs[filepath.Ext(p)].fn(mf, out, in, nil); err != nil {
					errs <- err
					return
				}
				out.Sync()

			case MarkdownExts[filepath.Ext(p)]:
				// Get layout template.
				ltmpl, ok := dirLayout.m[filepath.Dir(p)]
				if !ok {
					var err error
					ltmpl, err = template.ParseFiles(filepath.Join(filepath.Dir(p), "layout.tmpl"))
					if err != nil {
						errs <- err
						return
					}
					dirLayout.Lock()
					dirLayout.m[filepath.Dir(p)] = ltmpl
					dirLayout.Unlock()
				}
				// Create index.html in a directory with same name in build.
				rem, err := filepath.Rel(src, p)
				if err != nil {
					errs <- err
					return
				}
				f, err := createFile(filepath.Join(build, stripExt(rem), "index.html"))
				if err != nil {
					errs <- err
					return
				}
				defer f.Close()

				w := mf.Writer("text/html", f)
				defer w.Close()
				if err := ltmpl.Execute(w, TemplateArgs{
					Current: filePage[p],
					Dir:     dirPages[filepath.Dir(p)],
					All:     dirPages,
				}); err != nil {
					// TODO(nishanths): Fix this check. Appears to be issue
					// with minify package.
					if err != io.ErrClosedPipe {
						errs <- err
						return
					}
				}
				f.Sync()

			case filepath.Ext(p) == ".html":
				// Create corresponding .html file in build and
				// execute as template.
				tmpl, err := template.ParseFiles(p)
				if err != nil {
					errs <- err
					return
				}
				rem, err := filepath.Rel(src, p)
				if err != nil {
					errs <- err
					return
				}
				f, err := createFile(filepath.Join(build, rem))
				if err != nil {
					errs <- err
					return
				}
				defer f.Close()

				rel, err := filepath.Rel(filepath.Join(".", "src"), p)
				if err != nil {
					errs <- err
					return
				}

				w := mf.Writer("text/html", f)
				defer w.Close()
				if err := tmpl.Execute(w, TemplateArgs{
					Dir: dirPages[rel],
					All: dirPages,
				}); err != nil {
					// TODO(nishanths): Fix this check. Appears to be issue
					// with minify package.
					if err != io.ErrClosedPipe {
						errs <- err
						return
					}
				}
				f.Sync()

			default:
				// All other files - simply copy.
				rem, err := filepath.Rel(src, p)
				if err != nil {
					errs <- err
					return
				}
				errs <- copyFile(filepath.Join(build, rem), p)
			}
		}()
		return nil
	})

	if err != nil {
		return err
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}
