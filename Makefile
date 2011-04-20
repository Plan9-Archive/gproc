# Copyright 2010 The Go Authors.  All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

include $(GOROOT)/src/Make.inc

TARG=gproc_$(GOOS)_$(GOARCH)
GOFILES=\
	bproc_$(GOOS).go\
	bproc_$(GOOS)_$(GOARCH).go\
	common.go\
	except.go\
	info.go\
	locale.go\
	mexec.go\
	main.go\
	master.go\
	misc.go \
	slave.go\
	kanecfg.go \
	kfcfg.go \
	localcfg.go\
	strongboxcfg.go\
	jaguarcfg.go\
	jsoncfg.go\
	etchostscfg.go\

include $(GOROOT)/src/Make.cmd

all:	$(TARG) startkf startkane

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

startstrongbox: startstrongbox.go
	bash -c printenv

startkf:	startkf.go
	$(GC) startkf.go
	$(LD) -o startkf startkf.$(O)
	rm startkf.$(O)

startkane:	startkane.go
	$(GC) startkane.go
	$(LD) -o startkane startkane.$(O)
	rm startkane.$(O)

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
testlinuxp: $(TARG)
		./startgproc.sh -p3  10.12.0.11 10.12.0 12-17
testlinuxpd: $(TARG)
		./startgproc.sh -p3 -d8  10.12.0.11 10.12.0 12-17
testlinux1: $(TARG)
	./startgproc.sh -r -d10 10.12.0.11 10.12.0 12-12
