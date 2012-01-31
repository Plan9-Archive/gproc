// Copyright 2009 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ldd

import (
	"fmt"
	"testing"
)

type lddTest struct {
	file string
	root string
	libs string
	err  string
}

var lddTests = []lddTest{
	{"/bin/date", "/", "/lib:/usr/lib", ""},
}

func TestLdd(t *testing.T) {

	for _, tt := range lddTests {
		var _ error
		res, _ := Lddroot(tt.file, tt.root, tt.libs)
		fmt.Printf("Test: '%v' '%v' '%v' '%v' = '%v'\n", tt.file, tt.root, tt.libs, tt.err, res)
		/*
			if err != nil {
				t.Errorf("Test: '%v' '%v' '%v' '%v' = '%v': FAIL\n", tt.file, tt.root, tt.libs, tt.err, res)
			}
		*/
	}
}
