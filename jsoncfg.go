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
	"fmt"
	"os"
	"io/ioutil"
	"json"
	"log"
)

/*
	so what do you need to make this work?
	I need to make it so that I read the json file and I know my parent and my own address. 

	Not a big deal. In the vmatic environment perl builds the files. So let's keep it simple. 
	{"parentAddr":"1.1.1.1","ip":"2.2.2.2","addr":"3.3.3.3:3030","hostMap":null,"idMap":null}`
*/

func init() {
	addLocale("json", new(JsonCfg))
}

type JsonCfg struct {
	AparentAddr string
	Aaddr       string
	Aip         string
	idmap       map[string]string
}

func (l *JsonCfg) ConfigFrom(path string) (err os.Error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &l)
	if err != nil {
		fmt.Print("bad json ", err)
	}
	return
}


func (l *JsonCfg) Init(role string) {
	switch role {
	case "master", "slave":
	case "client", "run":
	}
}

func (l *JsonCfg) ParentAddr() string {
	return l.AparentAddr
}

func (l *JsonCfg) Addr() string {
	return l.Aaddr
}

func (l *JsonCfg) Ip() string {
	return l.Aip
}

func (s *JsonCfg) SlaveIdFromVitalData(vd *vitalData) (id string) {
	log.Fatal("Implement SlaveIdFromVitalData")
	return "1"
}

func (loc *JsonCfg) RegisterServer(l Listener) (err os.Error) {
	return
}
