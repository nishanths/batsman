package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	texttemplate "text/template"
	"time"

	"github.com/russross/blackfriday"
)

type Build struct {
	WorkDir string // Absolute path of work dir.
}

var markdownExts = map[string]bool{
	".md":       true,
	".markdown": true,
}

type TemplateArgs struct {
	Current *Page   // Current file.
	Dir     []*Page // All files in the same directory.
	All     map[string][]*Page
}

type Page struct {
	Content template.HTML // HTML content generated from markdown.
	Title   string        // Title from front matter.
	Time    time.Time     // Timestamp from front matter.
	Path    string        // HTTP path.
}

func (p *Page) FormatTime(layout string) string {
	return p.Time.Format(layout)
}

type ByTime []*Page

func (a ByTime) Len() int           { return len(a) }
func (a ByTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTime) Less(i, j int) bool { return a[i].Time.Before(a[j].Time) }

func (b *Build) makePages(root string) (map[string]*Page, map[string][]*Page, error) {
	mx := sync.Mutex{}
	ret := make(map[string]*Page)
	type result struct {
		Dir  string
		Page *Page
		Err  error
	}
	wg := sync.WaitGroup{}
	results := make(chan result)

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !markdownExts[filepath.Ext(p)] {
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
				t, err := texttemplate.New("content").Funcs(plugins).Parse(string(contents))
				if err != nil {
					results <- result{Err: err}
					return
				}
				if err := t.Execute(&buf, nil); err != nil {
					results <- result{Err: err}
					return
				}
				page.Content = template.HTML(
					blackfriday.MarkdownCommon(stripFrontMatter(buf.Bytes())),
				)
			}()

			fm, ok, err := ParseFrontMatter(bytes.NewReader(contents))
			if err != nil {
				results <- result{Err: err}
				return
			}
			if ok {
				page.Title = fm.Title
				page.Time = fm.Time
			} else {
				page.Time = info.ModTime()
			}
			if fm.Draft {
				return
			}

			innerWg.Wait()
			mx.Lock()
			ret[p] = page
			mx.Unlock()
			rel, err := filepath.Rel(b.WorkDir, p)
			if err != nil {
				results <- result{Err: err}
				return
			}
			s := strings.TrimPrefix(rel, "src"+string([]rune{filepath.Separator}))
			page.Path = "/" + filepath.ToSlash(changeExt(s, ".html"))
			results <- result{filepath.Dir(s), page, nil}
		}()

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	all := make(map[string][]*Page)
	for r := range results {
		if r.Err != nil {
			err = r.Err
		}
		all[r.Dir] = append(all[r.Dir], r.Page)
	}
	for k := range all {
		sort.Sort(ByTime(all[k]))
	}
	return ret, all, err
}

// changeExt switches the file extension in s to newExt.
// newExt is expected to start with ".". For example, ".txt".
// If s does not have a file extension, newExt is simply appended to s.
func changeExt(s, newExt string) string {
	return strings.TrimSuffix(s, filepath.Ext(s)) + newExt
}

func (b *Build) Run() error {
	src := filepath.Join(b.WorkDir, "src")
	build := filepath.Join(b.WorkDir, "build")

	filePage, dirPages, err := b.makePages(src)
	if err != nil {
		return WrapError{err}
	}

	// dirLayout is a map from directory name to the layout template for the
	// directory.
	dirLayout := struct {
		sync.Mutex
		m map[string]*template.Template
	}{m: make(map[string]*template.Template)}

	wg := sync.WaitGroup{}
	errs := make(chan error)
	err = filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		wg.Add(1)
		go func() {
			defer wg.Done()

			switch {
			case info.IsDir() || info.Name() == "layout.tmpl":
				return

			case markdownExts[filepath.Ext(p)]:
				// Get layout template.
				tmpl, ok := dirLayout.m[filepath.Dir(p)]
				if !ok {
					var err error
					tmpl, err = template.ParseFiles(filepath.Join(filepath.Dir(p), "layout.tmpl"))
					if err != nil {
						errs <- err
						return
					}
					dirLayout.Lock()
					dirLayout.m[filepath.Dir(p)] = tmpl
					dirLayout.Unlock()
				}
				// Create file with same name and .html extension in build.
				rem, err := filepath.Rel(src, p)
				if err != nil {
					errs <- err
					return
				}
				buf := bytes.Buffer{}
				f, err := createFile(changeExt(filepath.Join(build, rem), ".html"))
				if err != nil {
					errs <- err
					return
				}
				defer f.Close()

				if err := tmpl.Execute(&buf, TemplateArgs{
					Current: filePage[p],
					Dir:     dirPages[filepath.Dir(p)],
					All:     dirPages,
				}); err != nil {
					errs <- err
					return
				}

				t, err := template.New("content").Parse(buf.String())
				if err != nil {
					errs <- err
					return
				}
				if err := t.Execute(f, nil); err != nil {
					errs <- err
					return
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
				rel, err := filepath.Rel(b.WorkDir, p)
				if err != nil {
					errs <- err
					return
				}
				s := strings.TrimPrefix(rel, "src"+string([]rune{filepath.Separator}))
				if err := tmpl.Execute(f, TemplateArgs{
					Dir: dirPages[s],
					All: dirPages,
				}); err != nil {
					errs <- err
					return
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
