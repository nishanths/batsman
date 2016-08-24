package main

var rawFiles = map[string][]byte{
	"src/index.html": []byte(`<!doctype html>
<title>Hello</title>
<link rel="stylesheet" href="/css/style.css"/>
<div class="container">
  <p>Hello, world. I am the index page.
</div>`),

	"src/blog/index.html": []byte(`<!doctype html>
<title>Blog</title>
<link rel="stylesheet" href="/css/style.css"/>
<div class="container">
  <ul>
  {{ range .Posts }}
    <li>
      <h2>{{ .Title }}</h2>
      <div>{{ .Date }}</div>
  {{ end }}
  </ul>
</div>`),

	"src/blog/usage.md": []byte(`---
title: Styx Usage
date: 2016-07-31 15:04:00 -0900
draft: false
---

# styx
{{ .Gist nishanths/foo }}`),

	"src/css/style.css": []byte(`
/* http://meyerweb.com/eric/tools/css/reset/ 
   v2.0 | 20110126
   License: none (public domain)
*/

html, body, div, span, applet, object, iframe,
h1, h2, h3, h4, h5, h6, p, blockquote, pre,
a, abbr, acronym, address, big, cite, code,
del, dfn, em, img, ins, kbd, q, s, samp,
small, strike, strong, sub, sup, tt, var,
b, u, i, center,
dl, dt, dd, ol, ul, li,
fieldset, form, label, legend,
table, caption, tbody, tfoot, thead, tr, th, td,
article, aside, canvas, details, embed, 
figure, figcaption, footer, header, hgroup, 
menu, nav, output, ruby, section, summary,
time, mark, audio, video {
	margin: 0;
	padding: 0;
	border: 0;
	font-size: 100%;
	font: inherit;
	vertical-align: baseline;
}
/* HTML5 display-role reset for older browsers */
article, aside, details, figcaption, figure, 
footer, header, hgroup, menu, nav, section {
	display: block;
}
body {
	line-height: 1;
}
ol, ul {
	list-style: none;
}
blockquote, q {
	quotes: none;
}
blockquote:before, blockquote:after,
q:before, q:after {
	content: '';
	content: none;
}
table {
	border-collapse: collapse;
	border-spacing: 0;
}

html {
	font-size: 16px;
}
body {
	font-family: "Open Sans", sans-serif;
	line-height: 1.5;
	color: #000;
	padding: 1rem 2rem;
}
.container {
	max-width: 44rem;
}`),
}
