package main

var indexRaw = []byte(`<!doctype html>
<title>Hello</title>
<link rel="stylesheet" href="/css/style.css"/>
<p>Hello, world. I am the index page.`)

var blogRaw = []byte(`<!doctype html>
<title>Blog</title>
<link rel="stylesheet" href="/css/style.css"/>
<ul>
{{ range .Posts }}
  <li>
    <h2>{{ .Title }}</h2>
    <div>{{ .Date }}</div>
{{ end }}
</ul>`)
