package main

import (
	"fmt"
	"os"
	"path"

	"github.com/termie/go-shutil"
)

// build generates the site at workdir
// into the "build" directory, removing it
// if it already exists.
func build(_ ...string) error {
	build := path.Join(flags.WorkDir, "build")
	tmpBuild := os.TempDir()

	if err := os.RemoveAll(build); err != nil {
		return fmt.Errorf("styx: build failed: %s", err)
	}

	shutil.CopyTree(flags.WorkDir, tmpBuild, &shutil.CopyTreeOptions{
		Ignore: func(_ string, _ []os.FileInfo) []string {
			// TODO: This should support globs/patterns.
			// Also, relative to which directory?
			return flags.Exclude
		},
	})

	return nil
}
