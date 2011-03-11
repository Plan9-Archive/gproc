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
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)


type kane struct {
	parentAddr string
	ip         string
	addr       string // consider a better name
}

func init() {
	addLocale("kane", new(kane))
}

/* convention: kane nodes are named "kn" */
func (s *kane) Init(role string) {
	if *parent == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Panic("No host name!")
		}
		_, addrs, err := net.LookupHost(hostname)
		if err != nil {
			log.Panic("Can not look up " + hostname)
		}
		*parent = addrs[0]
	}
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
		/* on kane there's only ever one.
		 * pick out the lowest-level octet.
		 */
		hostname, _ := os.Hostname()
		which, _ := strconv.Atoi(hostname[2:])
		thirdOctet := 30 + (which - 1) /240
		switch {
		case which%40 == 0:
			s.parentAddr = *parent + ":6666"
		default:
			rackMaster := ((which + 39)/40) * 40
			s.parentAddr = "10.1." + strconv.Itoa(thirdOctet) + "." + strconv.Itoa(int(rackMaster)) + ":6666"
		}
		s.ip = "10.1." + strconv.Itoa(thirdOctet) + "." + strconv.Itoa(which)
		s.addr = s.ip + ":" + cmdPort
	case "client":
	}
}

func (s *kane) ParentAddr() string {
	return s.parentAddr
}

func (s *kane) Addr() string {
	return s.addr
}

func (s *kane) Ip() string {
	return s.ip
}

func (s *kane) SlaveIdFromVitalData(vd *vitalData) (id string) {
	addrs := strings.Split(vd.ServerAddr, ":", 2)
	octets := strings.Split(addrs[0], ".", 4)
	which, _ := strconv.Atoi(octets[3])
	/* get the lowest octet, take it mod 40 */
	if which%40 == 0 {
		id = strconv.Itoa(which / 40)
	} else {
		id = strconv.Itoa(which % 40)
	}
	return
}

func (s *kane) RegisterServer(l Listener) (err os.Error) {
	return
}
