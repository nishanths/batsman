package main

import (
	"bufio"
	"bytes"
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
// The hh:mm:ss and time zone are optional when parsing with
// ParseFrontMatter.
type FrontMatter struct {
	Draft bool
	Title string
	Time  time.Time
}

// String returns a representation that matches the front matter
// representation in a file.
func (fm *FrontMatter) String() string {
	buf := bytes.Buffer{}
	buf.WriteString(FrontMatterSep + "\n")
	if fm.Title != "" {
		buf.WriteString(fmt.Sprintf("title %s %q\n", frontMatterFieldSep, fm.Title))
	}
	if fm.Draft {
		buf.WriteString(fmt.Sprintf("draft %s %t\n", frontMatterFieldSep, fm.Draft))
	}
	buf.WriteString(fmt.Sprintf("time %s %q\n", frontMatterFieldSep, fm.Time.Format(defaultTimeFormat)))
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

var knownTimeFormats = []string{
	"2006-01-02 15:04:05 -07:00",
	"2006-01-02 15:04:05",
	"2006-01-02",
}
var defaultTimeFormat = knownTimeFormats[0]

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
		f.Time = currentTime
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

// FrontMatterSep is the separator between front matter
// and content.
const FrontMatterSep = `+++`

// FrontMatterSepBytes is FrontMatterSep as []byte.
var FrontMatterSepBytes = []byte(FrontMatterSep)

var frontMatterFieldSep = `=`

// ParseFrontMatter parses front matter from r.
func ParseFrontMatter(r io.Reader) (fm FrontMatter, exists bool, err error) {
	scanner := bufio.NewScanner(r)
	ok := scanner.Scan()
	if !ok {
		return
	}
	first := scanner.Text()
	if first != FrontMatterSep {
		return // No front matter.
	}
	exists = true

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
			break // end of front matter.
		}

		res := strings.SplitN(line, frontMatterFieldSep, 2)
		if len(res) != 2 {
			err = fmt.Errorf("styx: error: front matter %q should be in format \"key %s val\"", frontMatterFieldSep, line)
			return
		}
		key, val := clean(res[0]), clean(res[1])
		m[key] = val
	}

	err = fm.fromMap(m)
	return
}

// stripFrontMatter removes front matter (if any) from the input
// and returns the result.
//
// The function works on []byte to facililate working with
// blackfriday functions.
func stripFrontMatter(b []byte) []byte {
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
