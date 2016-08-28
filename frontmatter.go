package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

// FrontMatter represents front matter at the top
// of markdown files.
//
// Example front matter:
//
//   +++
//   time = "2006-01-02 15:04:05 -07:00"
//   title = "Hello, world"
//   draft = true
//   +++
//
type FrontMatter struct {
	Draft bool
	Title string
	Time  time.Time
}

// FrontMatterSep is the separator between front matter
// and content.
const FrontMatterSep = `+++`

// FrontMatterSepBytes is FrontMatterSep as []byte.
var FrontMatterSepBytes = []byte(FrontMatterSep)

// FrontMatterFieldSep is the separator between key and value.
const FrontMatterFieldSep = ` = `

// KnownTimeFormats is the the accepted time formats for time
// in front matter.
var KnownTimeFormats = []string{
	"2006-01-02 15:04:05 -07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}
var defaultTimeFormat = KnownTimeFormats[0]

// String returns a representation that matches the front matter
// representation in a file.
func (fm *FrontMatter) String() string {
	buf := bytes.Buffer{}
	buf.WriteString(FrontMatterSep + "\n")
	if fm.Title != "" {
		buf.WriteString(fmt.Sprintf("title %s %q\n", FrontMatterFieldSep, fm.Title))
	}
	if fm.Draft {
		buf.WriteString(fmt.Sprintf("draft %s %t\n", FrontMatterFieldSep, fm.Draft))
	}
	buf.WriteString(fmt.Sprintf("time %s %q\n", FrontMatterFieldSep, fm.Time.Format(defaultTimeFormat)))
	buf.WriteString(FrontMatterSep + "\n")
	return buf.String()
}

// InvalidFrontMatterError represents an error
// in a line of front matter.
type InvalidFrontMatterError struct {
	Key, Val    string
	CorrectVals []string
}

func (e *InvalidFrontMatterError) Error() string {
	s := fmt.Sprintf("styx: error: key %q has invalid value %q", e.Key, e.Val)
	if len(e.CorrectVals) > 0 {
		s += fmt.Sprintf(
			"\nexpected values/formats: {%s}", strings.Join(e.CorrectVals, ", "),
		)
	}
	return s
}

func (f *FrontMatter) fromMap(m map[string]string) error {
	v := m["draft"]
	if v == "true" {
		f.Draft = true
	} else if v != "" && v != "false" {
		return &InvalidFrontMatterError{"draft", v, []string{"true", "false"}}
	}

	f.Title = m["title"]

	if m["time"] != "" {
		for i, format := range KnownTimeFormats {
			t, err := time.Parse(format, v)
			if err == nil {
				f.Time = t
				break
			}
			if i == len(KnownTimeFormats)-1 {
				return &InvalidFrontMatterError{"time", v, KnownTimeFormats}
			}
		}
	}

	return nil
}

var ErrNoFrontMatter = errors.New("no front matter")

// Parse parses front matter in r.
// If r is empty or there is no front matter, the error
// will be ErrNoFrontMatter.
func (fm *FrontMatter) Parse(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	ok := scanner.Scan()
	if !ok {
		return ErrNoFrontMatter
	}
	first := scanner.Text()
	if first != FrontMatterSep {
		return ErrNoFrontMatter
	}

	m := map[string]string{
		"draft": "",
		"title": "",
		"time":  "",
	}
	clean := func(s string) string {
		return strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(s), `"`), `"`)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == FrontMatterSep {
			break // End of front matter.
		}

		res := strings.SplitN(line, FrontMatterFieldSep, 2)
		if len(res) != 2 {
			return fmt.Errorf("styx: error: front matter %q should be in format \"key%sval\"", line, FrontMatterFieldSep)
		}
		key, val := clean(res[0]), clean(res[1])
		m[key] = val
	}

	return fm.fromMap(m)
}

// trimFrontMatter removes front matter (if any) from the input
// and returns the result.
//
// The function works on []byte to facililate working with
// blackfriday functions.
func trimFrontMatter(b []byte) []byte {
	if !bytes.HasPrefix(b, FrontMatterSepBytes) {
		return b
	}
	ret := b[len(FrontMatterSepBytes):]
	idx := bytes.Index(ret, FrontMatterSepBytes)
	if idx == -1 {
		return b
	}
	return bytes.TrimLeftFunc(ret[idx+len(FrontMatterSepBytes):], unicode.IsSpace)
}
