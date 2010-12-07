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
	./startgproc.sh -d10 -r

testhome: $(TARG)
	./startgproc.sh -d10 -r 192.168.2.1 192.168.2 3-4,6-10

testtrace: $(TARG)
		./startgproc.sh -s -r 192.168.2.1 192.168.2 3-4,6-10

testtrace1: $(TARG)
		./startgproc.sh -s -r 192.168.2.1 192.168.2 3-3

smoketest: $(TARG)
	(cd testdata; ./test.sh)

testlocal: $(TARG)
	rm -f /tmp/g && ./gproc -debug=8 master /tmp/g &
	sleep 3
	./gproc  -debug=8 worker tcp4 127.0.0.1:`cat /tmp/srvaddr | sed 's/^.*://g'` 127.0.0.1:0 &
	sleep 3
	rm -rf /tmp/xproc
	time ./gproc -debug=8 exec /tmp/g tcp4 127.0.0.1:0 1 /bin/date
	rm -rf /tmp/xproc
	time ./gproc -debug=8 exec /tmp/g tcp4 127.0.0.1:0 1 /bin/date
	rm -f /tmp/g

testlinuxd: $(TARG)
	./startgproc.sh -d8  10.12.0.11 10.12.0 12-17
	#./startgproc.sh -r -d10 10.12.0.11 10.12.0 12-17
testlinux: $(TARG)
	./startgproc.sh  10.12.0.11 10.12.0 12-17
testlinux1: $(TARG)
	./startgproc.sh -r -d10 10.12.0.11 10.12.0 12-12