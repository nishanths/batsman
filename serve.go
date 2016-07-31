package main

import (
	"net/http"
	"path"
)

func serve(args ...string) error {
	srv := http.FileServer(http.Dir(path.Join(flags.WorkDir, "build")))
	return http.ListenAndServe(flags.Http, nil)
}
