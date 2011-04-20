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
	"os"
	"strings"
	"net"
)


type etchosts struct {
	parentAddr string
	ip         string
	addr       string // consider a better name
}

func init() {
	addLocale("etchosts", new(etchosts))
}

func (s *etchosts) Init(role string) {
	if *parent == "" {
		addrs, err := net.LookupHost("master")
		if err != nil {
			log.Panic("couldn't look up master in /etc/hosts!")
		}
		for _, a := range addrs {
			if a != "127.1" && a != "127.0.0.1" {
				Dprint(2, "going with parent = ", a)
				*parent = a
			}
		}
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
		/*
		 * pick out the lowest-level octet.
		 */
		hostname, err := os.Hostname()
		if err != nil {
			log.Panic("No host name!")
		}
		addrs, err := net.LookupHost(hostname)
		Dprint(4, "got addrs = ", addrs)
		if err != nil {
			log.Panic("couldn't look up hostname in /etc/hosts!")
		}
		s.parentAddr = *parent + ":" + *cmdPort
		for _, a := range addrs {
			if a != "127.1" && a != "127.0.0.1" {
				s.ip = a
			}
		}
		s.addr = s.ip + ":" + *cmdPort
	case "client":
	}
}

func (s *etchosts) ParentAddr() string {
	return s.parentAddr
}

func (s *etchosts) Addr() string {
	return s.addr
}

func (s *etchosts) Ip() string {
	return s.ip
}

func (s *etchosts) SlaveIdFromVitalData(vd *vitalData) (id string) {
	Dprint(2, "ParentAddr = ", vd.ParentAddr, ", HostAddr = ", vd.HostAddr)
	addrs := strings.Split(vd.HostAddr, ":", 2)
	hosts, err := net.LookupAddr(addrs[0])
	if err != nil || len(hosts) < 1 {
		log.Panic("couldn't look up hostname!")
	}	
	hostname := hosts[0]
	i := strings.IndexAny(hostname, "1234567890")
	Dprintf(2, "got i = %d for hostname = %s\n", i, hostname)
	id = hostname[i:]
	return
}

func (s *etchosts) RegisterServer(l Listener) (err os.Error) {
	return
}
