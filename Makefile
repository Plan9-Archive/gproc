# Copyright 2010 The Go Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

TARG=gproc
GOFILES=\
	bproc_$(GOOS).go\
	bproc_$(GOOS)_$(GOARCH).go\
	main.go\

include $(GOROOT)/src/Make.cmd

test: $(TARG)
	./test.sh

smoketest: $(TARG)
	(cd testdata; ./test.sh)
