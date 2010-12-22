/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the Lesser Gnu Programming License, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

package main

import (
	"os"
	"net"
	"log"
	"strconv"
)


type strongbox struct {
	parentAddr string
	addr string
	hostMap map[string][]string
}

func init() {
	addLocale("strongbox", new(strongbox))
}

func (s *strongbox) getIPs() []string {
	hostName, err := os.Hostname()
	if err != nil {
		log.Exit(err)
	}
	if addrs, ok := s.hostMap[hostName]; ok {
		return addrs
	}
	_, addrs, err := net.LookupHost(hostName)
	if err != nil {
		log.Exit(err)
	}
	return addrs
}

func (s *strongbox) initHostTable() {
	s.hostMap = make(map[string][]string)
	for i := 0; i < 197; i++ {
		n := strconv.Itoa(i)
		host := "cn" + n
		ip := "10.0.0." + n
		s.hostMap[host] = []string{ip}
	}
}

func (s *strongbox) Init(role string) {
		s.initHostTable()
		addrs := s.getIPs()
		switch role {
		case "master":
			cmdPort = "6666"
			/* we hardwire this because the LocalAddr of a 
			 * connected socket has an address of 0.0.0.0 !!
			 */
			s.addr = "10.0.0.254:" + cmdPort
			s.parentAddr = ""
		case "slave":
			cmdPort = "6666"
			/* on strongbox there's only ever one.
			 * pick out the lowest-level octet.
			 */
			b := net.ParseIP(addrs[0]).To4()
			which := b[3]
			switch {
			case which%7 == 0:
				s.parentAddr = "10.0.0.254:6666"
			default:
				boardMaster := ((which + 6) / 7) * 7
				s.parentAddr = "10.0.0." + strconv.Itoa(int(boardMaster)) + ":6666"
			}
			s.addr = b.String() + ":" + cmdPort
		case "client", "run":
		}
}

func (s *strongbox) ParentAddr() string {
	return s.parentAddr
}

func (s *strongbox) Addr() string {
	return s.addr
}

func (s *strongbox) RegisterServer(l Listener) (err os.Error) {
	return
}
