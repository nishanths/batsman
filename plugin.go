package main

import (
	"errors"
	"html/template"
	texttemplate "text/template"
)

var plugins = texttemplate.FuncMap{
	"Gist": func(v ...interface{}) (template.HTML, error) {
		if len(v) == 0 {
			return "", errors.New("Gist: require at least one argument")
		}
		return "<script></script>", nil
	},
}
