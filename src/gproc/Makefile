# Copyright 2010 The Go Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

TARG=gproc_$(GOOS)_$(GOARCH)
GOFILES=\
	bproc_$(GOOS).go\
	bproc_$(GOOS)_$(GOARCH).go\
	common.go\
	info.go\
	mexec.go\
	main.go\
	master.go\
	misc.go \
	slave.go\
	web.go\

include $(GOROOT)/src/Make.cmd

all:	$(TARG)

