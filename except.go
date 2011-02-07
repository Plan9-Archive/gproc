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
	"log"
)

func except(masterAddr string, files []string) (exceptOK *Resp) {
	req := StartReq{Command: "x", Args: files}
	log.SetPrefix("except " + *prefix + ": ")
	client, err := Dial("unix", "", masterAddr)
	if err != nil {
		log.Fatal("startExecution: dialing: ", masterAddr, " ", err)
	}
	r := NewRpcClientServer(client)

	/* master sends us vital data */
	var vitalData vitalData
	exceptOK = &Resp{}
	r.Recv("vitalData", &vitalData)

	r.Send("exceptFiles", req)
	r.Recv("exceptOK", exceptOK)
	Dprintln(3, "except: finished: ", *exceptOK)
	return
}
