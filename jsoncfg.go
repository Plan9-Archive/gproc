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
	"io/ioutil"
	"json"
	"log"
)

/*

so what do you need to make this work?
I need to make it so that I read the json file and I know my parent and my own address. 
how do I get that? 
where is l.Addr set?

so I just need to find my own address in local and read it in. 

so how do you do that?
you just push the addresses out. 

how do I get my own address? 
I read it 

how do you find out your own ip address? 

easy you write it in. 

and you find out all of your addresses.
Which brings up an interesting problem, unices don't make it easy to get your addresses. 
Will probably need to do an ioctl version using SIOCGIFADDR

you need to be able to find your addresses in osx and linux. 

that is a good question. 

{"candidates":[
{"addr":192.168.2.3", "parentAddr":"192.168.2.1"},
{"addr":192.168.2.4", "parentAddr":"192.168.2.1"},
{"addr":192.168.2.6", "parentAddr":"192.168.2.1"},
{"addr":192.168.2.7", "parentAddr":"192.168.2.1"},
{"addr":192.168.2.8", "parentAddr":"192.168.2.1"},
{"addr":192.168.2.9", "parentAddr":"192.168.2.1"},
{"addr":192.168.2.10", "parentAddr":"192.168.2.1"},
]}


*/


func init() {
	addLocale("json", new(JsonCfg))
}

type JsonCfg struct{
	parentAddr string
	addr string
	ip string
	candidates []map[string]string
}

func (l *JsonCfg) ConfigFrom(path string) (err os.Error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &l.candidates)
	log.Print("candidates:")
	for _, v := range l.candidates {
		log.Println(v["addr"])
	}
	log.Print("done candidates")
	return
}


func (l *JsonCfg) Init(role string) {
	getIfc()
	switch role {
	case "master", "slave":
		l.parentAddr = "127.0.0.1:2424"
	case "client", "run":
	}
}

func (l *JsonCfg) ParentAddr() string {
	return l.parentAddr
}

func (l *JsonCfg) Addr() string {
	return l.addr
}

func (l *JsonCfg) Ip() string {
	return l.addr
}

func (s *JsonCfg) SlaveIdFromVitalData(vd *vitalData) (id string) {
	log.Exit("Implement SlaveIdFromVitalData")
	return "1"
}

func (loc *JsonCfg) RegisterServer(l Listener) (err os.Error) {
	return
}

