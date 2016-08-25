package main

import (
	"bytes"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	All     []*Page // All files in the same directory.
}

type Page struct {
	Content template.HTML // HTML content generated from markdown.
	Title   string        // Title from front matter.
	Time    time.Time     // Timestamp from front matter.
}

func makeAllPages(root string) (map[string]*Page, map[string][]*Page, error) {
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

			b, err := ioutil.ReadFile(p)
			if err != nil {
				results <- result{Err: err}
				return
			}
			b = stripFrontMatter(b)

			page := &Page{}

			innerWg := sync.WaitGroup{}
			innerWg.Add(1)
			go func() {
				defer innerWg.Done()
				page.Content = template.HTML(blackfriday.MarkdownCommon(b))
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
			ret[p] = page
			results <- result{p, page, nil}
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
	return ret, all, err
}

func changeExt(s, newExt string) string {
	return strings.TrimSuffix(s, filepath.Ext(s)) + newExt
}

func (b *Build) Run() error {
	src := filepath.Join(b.WorkDir, "src")
	build := filepath.Join(b.WorkDir, "build")

	filePage, dirPages, err := makeAllPages(src)
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
				f, err := createFile(changeExt(filepath.Join(build, rem), ".html"))
				if err != nil {
					errs <- err
					return
				}
				defer f.Close()
				// Execute template into .html file.
				if err := tmpl.Execute(f, TemplateArgs{
					Current: filePage[p],
					All:     dirPages[filepath.Dir(p)],
				}); err != nil {
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
				if err := tmpl.Execute(f, TemplateArgs{
					All: dirPages[filepath.Dir(p)],
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
