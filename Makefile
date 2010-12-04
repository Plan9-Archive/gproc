# Copyright 2010 The Go Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

TARG=gproc
GOFILES=\
	bproc_$(GOOS).go\
	bproc_$(GOOS)_$(GOARCH).go\
	common.go\
	mexec.go\
	main.go\
	master.go\
	run.go\
	slave.go\
	#	graphviz.go\

include $(GOROOT)/src/Make.cmd

testwork: $(TARG)
	./startgproc.sh

testhome: $(TARG)
	./startgproc.sh -d10 -r 192.168.2.1 192.168.2 3-4,6-10

smoketest: $(TARG)
	(cd testdata; ./test.sh)
