package main

import (
	"bytes"
	"testing"
)

func TestStripFrontMatter(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		in, expected []byte
	}{
		{
			[]byte(`+++
title = foo
+++
# bar`),
			[]byte(`# bar`),
		},

		{
			[]byte(`# bar`),
			[]byte(`# bar`),
		},
	}

	for _, tc := range testcases {
		res := trimFrontMatter(tc.in)
		if !bytes.Equal(res, tc.expected) {
			t.Fatalf("trimFrontMatter: got %s, expected %s", res, tc.expected)
		}
	}
}
