/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the GNU Lesser General Public License, version 2, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

package main

import (
	"os"
	"strconv"
	"strings"
)


type strongbox struct {
	parentAddr string
	ip         string
	addr       string // consider a better name
}

func init() {
	addLocale("strongbox", new(strongbox))
}

/* convention: strongbox nodes are named "cn" */
func (s *strongbox) Init(role string) {
	switch role {
	case "master":
		cmdPort = "6666"
		/* we hardwire this because the LocalAddr of a 
		 * connected socket has an address of 0.0.0.0 !!
		 */
		s.ip = *parent
		s.addr = s.ip + ":" + cmdPort
		s.parentAddr = ""
	case "slave", "run":
		cmdPort = "6666"
		/* on strongbox there's only ever one.
		 * pick out the lowest-level octet.
		 */
		hostname, _ := os.Hostname()
		which, _ := strconv.Atoi(hostname[2:])
		switch {
		case which%7 == 0:
			s.parentAddr = *parent + ":6666"
		default:
			boardMaster := ((which + 6) / 7) * 7
			s.parentAddr = "10.0.0." + strconv.Itoa(int(boardMaster)) + ":6666"
		}
		s.ip = "10.0.0." + strconv.Itoa(which)
		s.addr = s.ip + ":" + cmdPort
	case "client":
	}
}

func (s *strongbox) ParentAddr() string {
	return s.parentAddr
}

func (s *strongbox) Addr() string {
	return s.addr
}

func (s *strongbox) Ip() string {
	return s.ip
}

func (s *strongbox) SlaveIdFromVitalData(vd *vitalData) (id string) {
	/* grab the server address from vital data and index into our map */
	addrs := strings.Split(vd.ServerAddr, ":", 2)
	octets := strings.Split(addrs[0], ".", 4)
	which, _ := strconv.Atoi(octets[3])
	/* get the lowest octet, take it mod 7 */
	if which%7 == 0 {
		id = strconv.Itoa(which / 7)
	} else {
		id = strconv.Itoa(which % 7)
	}
	return
}

func (s *strongbox) RegisterServer(l Listener) (err os.Error) {
	return
}
