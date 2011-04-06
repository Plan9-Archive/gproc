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
	"strings"
	"os"
)

var id string

/* We will for now assume that addressing is symmetric, that is, if we Dial someone on
 * a certain address, that's the address they should Dial us on. This assumption has held
 * up well for quite some time. And, in fact, it makes no sense to do it any other way ...
 */
/* note that we're going to be able to merge master and slave fairly soon, now that they do almost the same things. */
func startSlave(fam, masterAddr string, loc Locale) {
	/* slight difference from master: we're ready when we start, since we run things */
	vitalData := &vitalData{HostReady: true}
	/* some simple sanity checking */
	if *DoPrivateMount == true && os.Getuid() != 0 {
		log.Fatal("Need to run as root for private mounts")
	}
	client, err := Dial(fam, "", masterAddr)
	if err != nil {
		log.Fatal("dialing:", err)
	}

	/* vitalData -- what we're doing here is assembling information for our parent. 
	 * we have to tell our parent what port we look for process startup commands on, 
	 * the address of our side of the Dial connection, and, due to a limitation in the Unix
	 * kernels going back a long time, we might as well tell the master its own address for
	 * the socket, since *the master can't get it*. True! 
	 */
	addr := strings.Split(client.LocalAddr().String(), ":", -1)
	peerAddr := addr[0] + ":0"
	vitalData.ServerAddr = newListenProc("slaveProc", slaveProc, peerAddr)
	vitalData.HostAddr = client.LocalAddr().String()
	vitalData.ParentAddr = client.RemoteAddr().String()
	r := NewRpcClientServer(client, *binRoot)
	//Dprint(2, "DestDir = ", r.d.DestDir)
	initSlave(r, vitalData)
	go registerSlaves(loc)
	for {
		/* make sure the directory exists and then do the private name space mount */
		/* there are enough pathological cases to deal with here that it doesn't hurt to 
		 * do the mkdir each time. Even though, abusive users can screw us: 
		 * suppose they run rm -rf /tmp/xproc. Nothing is perfect. 
		 */
		os.Mkdir(*binRoot, 0700)
		if *DoPrivateMount == true {
			doPrivateMount(*binRoot)
		}
		/* don't ever make this 'go slaveProc'. This really needs to be synchronous lest you 
		 * privatize the name space out from under yourself. It makes some sense: you really 
		 * want to serialize on receiving the packet, unpacking it, and then forking the kid. 
		 * Once the child is running you have no further worries. 
		 * what's interesting is to think about whether we should fork a gproc e for any 
		 * 'relay' uses. 
		 */
		slaveProc(r)
	}
}

func initSlave(r *RpcClientServer, v *vitalData) {
	Dprint(2, "initSlave: ", v)
	r.Send("startSlave", *v)
	resp := &SlaveResp{}
	r.Recv("startSlave", &resp)
	id = resp.Id
	log.SetPrefix("slave " + id + ": ")
}

func slaveProc(r *RpcClientServer) {
	req := &StartReq{}
	r.Recv("slaveProc", req)
	Dprint(2, "slaveProc: req ", *req)
	go runLocal(req)
	/* the child may end before we even get here, but since we still own this name 
	 * space, the files are still there. 
	 */
	nnodes := sendCommandsToNodes(r, req, *binRoot)
	Dprint(2, "Sent to ", nnodes, " nodes")

}

func RunChild(req *StartReq) (nsend *nodeExecList){
	Dprintln(2, "ForkRelay: ", req.Nodes, " fileServer: ", req)
	/* create the array of strings to send. You can't just send the slaveinfo struct as Go won't like that. 
	 * You don't have fork
	 * and you can't do it here as the child will build a private name space. 
	 * So take the req.Nodes, bust them into bits just as the master does, and create an array of 
	 * socket names {'"a.b.c.d/x"...} and the subnode names {"1-5"} and pass them down. 
	 * this is almost ready but it won't make it.
	 */
	ne, _ := parseNodeList(req.Nodes)
	nsend = &nodeExecList{Subnodes: ne[0].Subnodes}
	nsend.Nodes = slaves.ServIntersect(ne[0].Nodes)
	Dprint(2, "Parsed node list to ", ne, " and nsend is ", nsend)
	return nsend
	/* for now; later this will be a call to cache files etc. etc. */
}
