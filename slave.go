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
	"net"
	"fmt"
	"gob"
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
	Dprint(2, "dialing masterAddr ", masterAddr)
	master, err := Dial(fam, "", masterAddr)
	if err != nil {
		log.Fatal("dialing:", err)
	}

	/* vitalData -- what we're doing here is assembling information for our parent. 
	 * we have to tell our parent what port we look for process startup commands on, 
	 * the address of our side of the Dial connection, and, due to a limitation in the Unix
	 * kernels going back a long time, we might as well tell the master its own address for
	 * the socket, since *the master can't get it*. True! 
	 */
	addr := strings.Split(master.LocalAddr().String(), ":", -1)
	peerAddr := addr[0] + ":0"

	laddr, _ := net.ResolveTCPAddr(peerAddr) // This multiple-return business sometimes gets annoying
	netl, err := net.ListenTCP(defaultFam, laddr) // this is how we're ditching newListenProc
	vitalData.ServerAddr = netl.Addr().String()
	vitalData.HostAddr = master.LocalAddr().String()
	vitalData.ParentAddr = master.RemoteAddr().String()
	r := NewRpcClientServer(master, *binRoot)
	initSlave(r, vitalData)
	go registerSlaves(loc)
	go func() {
		for {
			// Wait for a connection from the master
			c, err := netl.AcceptTCP()
			if err != nil {
				log.Fatal("problem in netl.Accept()")
			}
			Dprint(3, "Received connection from: ", c.RemoteAddr())

			// start a new process, give it 'c' as stdin.
			connFile, _ := c.File()
			readp, writep, _ := os.Pipe() // we'll send a list of slaves over this
			readp2, writep2, _ := os.Pipe() // the child will send a list of nodes and ask for a list of slaves
			f := []*os.File{connFile, readp, os.Stderr, writep2} // we can't use Stderr because the child wants to write to it
			cwd, _ := os.Getwd()
			procattr := os.ProcAttr{Env: nil, Dir: cwd, Files: f}
			argv := []string{
                "gproc",
                fmt.Sprintf("-debug=%d", *DebugLevel),
                fmt.Sprintf("-p=%v", *DoPrivateMount),
                fmt.Sprintf("-locale=%v", *locale),
				fmt.Sprintf("-binRoot=%v", *binRoot),	
                "-prefix=" + id,
                "R",
			}
			// Start the new process
			p, err := os.StartProcess(os.Args[0], argv, &procattr)
			if err != nil {
				log.Fatal("startSlave: ", err)
			} else {
				passrpc := &RpcClientServer{ E: gob.NewEncoder(writep), D: gob.NewDecoder(writep) }
				returnrpc := &RpcClientServer{ E: gob.NewEncoder(readp2), D: gob.NewDecoder(readp2) }

				var ne nodeExecList
				// This is the list of nodes the child got in its request
				returnrpc.Recv("startSlave getting nodes ", &ne)
				// The child doesn't have the slaves populated, so we have to do it
				ne.Nodes = slaves.ServIntersect(ne.Nodes)
				passrpc.Send("startSlave sending nodes ", ne)
				
				w, _ := p.Wait(0)
				Dprint(2, "startSlave: process returned ", w.String())
			}
			c.Close()
			writep.Close()
			readp2.Close()
		}
	}()

	// This read doesn't really matter, the important thing is that it will fail when the master goes away
	foo := &StartReq{}
	r.Recv("slaveProc done", &foo)
}

func initSlave(r *RpcClientServer, v *vitalData) {
	Dprint(2, "initSlave: ", v)
	r.Send("startSlave", *v)
	resp := &SlaveResp{}
	r.Recv("startSlave", &resp)
	id = resp.Id
	log.SetPrefix("slave " + id + ": ")
}

/*
 * Receive a StartReq (and, thanks to filemarshal, all the files we need), 
 * then go off and run the program with runLocal. While that's happening,
 * we set up ioproxies as needed, then forward on the StartReq we received
 * to any sub-nodes we may have.
 *
 * This is a bit of a beast, but it's more efficient than the last iteration.
 */
func slaveProc(r *RpcClientServer, inforpc *RpcClientServer, returnrpc *RpcClientServer) {
	//go registerSlaves(loc)
	os.Mkdir(*binRoot, 0700)
	if *DoPrivateMount == true {
		doPrivateMount(*binRoot)
	}
	done := make(chan int, 0)
	req := &StartReq{}
	r.Recv("slaveProc", &req)
	Dprint(2, "slaveProc: req ", *req)

	n, c, err := fileTcpDial(req.Lserver)
	if err != nil {
		log.Fatal("tcpDial: ", err)
	}

	go runLocal(req, c, n, done)
	/* the child may end before we even get here, but since we still own this name 
	 * space, the files are still there. 
	 */

	var nodesCopy string
	var l Listener
	var workerChan chan int
	var numWorkers int
	numWorkers = 0
	nodesCopy = req.Nodes
	slaveNodes, err := parseNodeList(nodesCopy)
	returnrpc.Send("send slaveNodes ", slaveNodes[0])
	var availableSlaves nodeExecList
	inforpc.Recv("recv availableSlaves", &availableSlaves)

	Dprint(2, "receiveCmds: sendReq.Nodes: ", req.Nodes, " expands to ", slaveNodes)
	if err != nil {
		return
	}

	if len(availableSlaves.Nodes) > 0 {
			workerChan, l, err = ioProxy(defaultFam, loc.Ip()+":0", c)
			if err != nil {
				log.Fatalf("slave: ioproxy: ", err)
			}
			Dprint(2, "netwaiter locl.Ip() ", loc.Ip(), " listener at ", l.Addr().String())
			req.Lfam = l.Addr().Network()
			req.Lserver = l.Addr().String()

			for _, _ = range availableSlaves.Nodes {
				numWorkers += 1
			}
	}	
	nnodes := sendCommandsToANode(req, slaveNodes[0], *binRoot, availableSlaves.Nodes)
	Dprint(2, "Sent to ", nnodes, " nodes")
	for numWorkers > 0 {
		worker := <-workerChan
		Dprint(2, worker, " returned, ", numWorkers, " workers left")
		numWorkers--
	}
	<-done
	c.Close()
	n.Close()
	Dprint(2, "Exiting slaveProc")
}

