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
	"strings"
	"io/ioutil"
	"log"
	"strconv"
)


type local struct {
	parentAddr string
	addr       string
	ip         string
	maxid      int
	idMap      map[string]string
}

func init() {
	addLocale("local", &local{"0.0.0.0:0", "0.0.0.0:0", "0.0.0.0", 0, make(map[string]string)})
}

func (l *local) Init(role string) {
	switch role {
	case "master", "slave":
		cmd, err := ioutil.ReadFile(srvAddr)
		if err != nil {
			log.Fatal(err)
		}
		l.parentAddr = "127.0.0.1:" + string(cmd)
	case "client", "run":
	}
}

func (l *local) ParentAddr() string {
	return l.parentAddr
}

func (l *local) Addr() string {
	return l.addr
}

func (l *local) Ip() string {
	return l.ip
}

func (s *local) SlaveIdFromVitalData(vd *vitalData) string {
	/* grab the server address from vital data and index into our map */
	addrs := strings.Split(vd.ServerAddr, ":", 2)
	id, ok := s.idMap[addrs[0]]
	if !ok {
		s.maxid++
		s.idMap[addrs[0]] = strconv.Itoa(s.maxid), ok
	}
	return id
}

func (loc *local) RegisterServer(l Listener) (err os.Error) {
	/* take the port only -- the address shows as 0.0.0.0 */
	addr := strings.Split(l.Addr().String(), ":", 2)
	return ioutil.WriteFile(srvAddr, []byte(addr[1]), 0644)
}
