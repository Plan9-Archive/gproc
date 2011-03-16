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

const (
	BLOCKSIZE int = 20
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
		*parent = "10.1.254.254"
	}
	switch role {
	case "master":
		/* we hardwire this because the LocalAddr of a 
		 * connected socket has an address of 0.0.0.0 !!
		 */
		s.ip = *parent
		s.addr = s.ip + ":" + *cmdPort
		s.parentAddr = ""
	case "slave", "run":
		/* on kane there's only ever one.
		 * pick out the lowest-level octet.
		 */
		hostname, _ := os.Hostname()
		which, _ := strconv.Atoi(hostname[2:])
		thirdOctet := 30 + (which - 1) /240

		/* Our KANE IPs go 10.1.30.1-240, .31.1-240, .32.1-40 */
		lastOctet := which
		if which / 241 > 0 {
			lastOctet = (which % 241) + 1
		}

		switch {
		case which%BLOCKSIZE == 0:
			s.parentAddr = *parent + ":" + *cmdPort
		default:
			//rackMaster := ((which + BLOCKSIZE-1)/BLOCKSIZE) * BLOCKSIZE
			rackMaster := ((lastOctet + BLOCKSIZE-1)/BLOCKSIZE) * BLOCKSIZE
			s.parentAddr = "10.1." + strconv.Itoa(thirdOctet) + "." + strconv.Itoa(int(rackMaster)) + ":" + *cmdPort
		}
		//s.ip = "10.1." + strconv.Itoa(thirdOctet) + "." + strconv.Itoa(which)
		s.ip = "10.1." + strconv.Itoa(thirdOctet) + "." + strconv.Itoa(lastOctet)
		s.addr = s.ip + ":" + *cmdPort
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
	thirdOctet, _ := strconv.Atoi(octets[2])
	/* get the lowest octet, take it mod 40 */
	if which%BLOCKSIZE == 0 {
		id = strconv.Itoa(which / BLOCKSIZE + (thirdOctet - 30)*(240/BLOCKSIZE))
	} else {
		id = strconv.Itoa(which % BLOCKSIZE)
	}
	return
}

func (s *kane) RegisterServer(l Listener) (err os.Error) {
	return
}
