/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the GNU Lesser General Public License, version 2, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

/* this is "kane flat", i.e. no intermediate masters, for testing */

package main

import (
	"log"
	"os"
	"strconv"
	"strings"
)


type kf struct {
	parentAddr string
	ip         string
	addr       string // consider a better name
}

func init() {
	addLocale("kf", new(kf))
}

/* convention: kf nodes are named "kn" */
func (s *kf) Init(role string) {
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
		/* on kf there's only ever one.
		 * pick out the lowest-level octet.
		 */
		hostname, err := os.Hostname()
		if err != nil {
			log.Panic("No host name!")
		}
		which, _ := strconv.Atoi(hostname[2:])
		thirdOctet := 30 + (which-1)/240
		//log.Printf("thirdOctet = %d", thirdOctet)
		fourthOctet := which % 240
		s.parentAddr = *parent + ":" + *cmdPort
		s.ip = "10.1." + strconv.Itoa(thirdOctet) + "." + strconv.Itoa(fourthOctet)
		s.addr = s.ip + ":" + *cmdPort
	case "client":
	}
}

func (s *kf) ParentAddr() string {
	return s.parentAddr
}

func (s *kf) Addr() string {
	return s.addr
}

func (s *kf) Ip() string {
	return s.ip
}

func (s *kf) SlaveIdFromVitalData(vd *vitalData) (id string) {
	addrs := strings.SplitN(vd.ServerAddr, ":", 2)
	octets := strings.SplitN(addrs[0], ".", 4)
	high, _ := strconv.Atoi(octets[2])
	low, _ := strconv.Atoi(octets[3])
	id = strconv.Itoa((high-30)*240 + low)
	return
}

func (s *kf) RegisterServer(l Listener) (err os.Error) {
	return
}
