package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
)

// FrontMatter represents front matter at the top
// of markdown files.
type FrontMatter struct {
	Draft bool
	Title string
	Time  time.Time
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
			"\nexpected values/formats are: {%s}", strings.Join(e.CorrectVals, ", "),
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

const FrontMatterSep = `---`

func ParseFrontMatter(r io.Reader) (fm FrontMatter, exists bool, err error) {
	scanner := bufio.NewScanner(r)
	ok := scanner.Scan()
	if !ok {
		return
	}
	first := scanner.Text()
	if first != FrontMatterSep {
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
		if line == FrontMatterSep {
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
