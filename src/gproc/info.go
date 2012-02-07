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
)

func getInfo(masterAddr, query string) (info *Resp) {
	req := StartReq{Command: "i"}
	log.SetPrefix("getIbfo " + *prefix + ": ")
	client, err := Dial("unix", "", masterAddr)
	if err != nil {
		log_error("startExecution: dialing: ", masterAddr, " ", err)
	}
	r := NewRpcClientServer(client, *binRoot)

	/* master sends us vital data */
	var vitalData vitalData
	info = &Resp{}
	if r.Recv("vitalData", &vitalData) != nil {
		log_error("Could not receive vital data")
	}
	if !vitalData.HostReady {
		log_info("No hosts yet: ", vitalData.Error)
		return
	}

	r.Send("getInfo", req)
	if r.Recv("getinfo", info) != nil {
		log_error("getinfo failed")
	}
	log_info("getInfo: finished: ", *info)
	return
}
