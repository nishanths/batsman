package main

import (
	"net/http"
	"path"
)

func serve(args ...string) error {
	stderr.Printf("serving on %s ...\n", flags.Http)
	return http.ListenAndServe(
		flags.Http,
		http.FileServer(http.Dir(path.Join(flags.WorkDir, "build"))),
	)
}
