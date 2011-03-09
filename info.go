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
	"log"
)

func getInfo(masterAddr, query string) (info *Resp) {
	req := StartReq{Command: "i"}
	log.SetPrefix("getIbfo " + *prefix + ": ")
	client, err := Dial("unix", "", masterAddr)
	if err != nil {
		log.Fatal("startExecution: dialing: ", masterAddr, " ", err)
	}
	r := NewRpcClientServer(client)

	/* master sends us vital data */
	var vitalData vitalData
	info = &Resp{}
	r.Recv("vitalData", &vitalData)
	if !vitalData.HostReady {
		fmt.Print("No hosts yet: ", vitalData.Error, "\n")
		return
	}

	r.Send("getInfo", req)
	r.Recv("getinfo", info)
	Dprintln(3, "getInfo: finished: ", *info)
	return
}
