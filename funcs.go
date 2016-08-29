package main

import (
	"errors"
	"fmt"
	"html/template"
	"net/url"
	texttemplate "text/template"
)

var funcs = texttemplate.FuncMap{
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
{{ Gist "user/123abcdef" }}
{{ Gist "user/123abcdef" "foo.rb" }}
{{ Gist "123abcedef" }}
{{ Gist "123abcedef" "bar.rb" }}`)
		}
	},
}
