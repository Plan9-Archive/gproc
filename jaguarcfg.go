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
)


type jaguar struct {
	parentAddr string
	addr       string
	ip         string
}

func init() {
	addLocale("jaguar", new(jaguar))
}

func (s *jaguar) initHostTable() {
}

func (s *jaguar) Init(role string) {
	switch role {
	case "master":
		cmdPort = "6666"
		/* we hardwire this because the LocalAddr of a 
		 * connected socket has an address of 0.0.0.0 !!
		 */
		s.addr = "192.168.30.69:" + cmdPort
		s.parentAddr = ""
	case "slave":
		cmdPort = "6666"
		s.parentAddr = "192.168.30.69:" + cmdPort
		s.ip = "0.0.0.0"
		s.addr = s.ip + ":" + cmdPort
	case "client", "run":
	}
}

func (s *jaguar) ParentAddr() string {
	return s.parentAddr
}

func (s *jaguar) Addr() string {
	return s.addr
}

func (s *jaguar) Ip() string {
	return s.ip
}

func (s *jaguar) SlaveIdFromVitalData(vd *vitalData) (id string) {
	log.Fatal("Implement SlaveIdFromVitalData")
	return "1"
}

func (s *jaguar) RegisterServer(l Listener) (err os.Error) {
	return
}
