package main

import (
	"errors"
	"fmt"
	"html/template"
	"net/url"
	texttemplate "text/template"
)

var plugins = texttemplate.FuncMap{
	"Gist": func(v ...interface{}) (template.HTML, error) {
		switch len(v) {
		case 1:
			return template.HTML(fmt.Sprintf("<script src=\"https://gist.github.com/%s.js\"></script>", v[0].(string))), nil
		case 2:
			return template.HTML(fmt.Sprintf("<script src=\"https://gist.github.com/%s.js?%s\"></script>",
				v[0].(string),
				url.Values{"file": {v[1].(string)}}.Encode(),
			)), nil
		default:
			return "", errors.New(`Gist: invalid arguments
valid examples:
{{ Gist "user/28949e1d5ee2273f9fd3" }}
{{ Gist "user/28949e1d5ee2273f9fd3" "foo.rb" }}
{{ Gist "28949e1d5ee2273f9fd3" }}
{{ Gist "28949e1d5ee2273f9fd3" "foo.rb" }}`)
		}
	},
}
