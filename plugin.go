package main

import "html/template"

var plugins = template.FuncMap{
	"Gist": func(v ...interface{}) template.HTML {
		return "<script></script>"
	},
}
