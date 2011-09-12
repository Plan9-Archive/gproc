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
		log.Fatal("startSlave: dialing:", err)
	}

	/* vitalData -- what we're doing here is assembling information for our parent. 
	 * we have to tell our parent what port we look for process startup commands on, 
	 * the address of our side of the Dial connection, and, due to a limitation in the Unix
	 * kernels going back a long time, we might as well tell the master its own address for
	 * the socket, since *the master can't get it*. True! 
	 */
	addr := strings.SplitN(master.LocalAddr().String(), ":", -1)
	peerAddr := addr[0] + ":0"

	laddr, _ := net.ResolveTCPAddr("tcp4", peerAddr)      // This multiple-return business sometimes gets annoying
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
			connFile, _ := c.File()                              // the new process will read a StartReq from connFile
			readp, writep, _ := os.Pipe()                        // we'll send a list of slaves over this
			readp2, writep2, _ := os.Pipe()                      // the child will send a list of nodes and ask for a list of slaves
			f := []*os.File{connFile, readp, os.Stderr, writep2} // we can't use Stderr because the child wants to write to it
			cwd, _ := os.Getwd()
			procattr := os.ProcAttr{Env: nil, Dir: cwd, Files: f}
			argv := []string{
				"gproc",
				fmt.Sprintf("-debug=%d", *DebugLevel),
				fmt.Sprintf("-p=%v", *DoPrivateMount),
				fmt.Sprintf("-locale=%v", *locale),
				fmt.Sprintf("-binRoot=%v", *binRoot),
				fmt.Sprintf("-parent=%v", *parent),
				"-prefix=" + id,
				"R", // "R" = run a program
			}
			// Start the new process
			p, err := os.StartProcess(os.Args[0], argv, &procattr)
			if err != nil {
				log.Fatal("startSlave: ", err)
			} else {
				// The process started, let's make some RpcClientServers on our end to communicate with it
				passrpc := &RpcClientServer{E: gob.NewEncoder(writep), D: gob.NewDecoder(writep)}
				returnrpc := &RpcClientServer{E: gob.NewEncoder(readp2), D: gob.NewDecoder(readp2)}

				var ne nodeExecList
				// This is the list of nodes the child got in its request
				if returnrpc.Recv("startSlave getting nodes ", &ne) != nil {
					return
				}
				// The child doesn't have the slaves populated, so we have to do it
				ne.Nodes = slaves.ServIntersect(ne.Nodes)
				passrpc.Send("startSlave sending nodes ", ne)

				w, _ := p.Wait(0) // Wait until the child process is finished. We need to do things sorta synchronously
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
	if r.Recv("startSlave", &resp) != nil {
		log.Fatal("Can't start slave")
	}
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
 *
 * There are 3 RpcClientServer arguments, because of the way things work.
 * 'r' is connected to this node's parent/master so we can read the StartReq
 * 'inforpc' is connected to the original slave process, which knows our slaves and can send us a nodeExecList
 * 'returnrpc' is used to ask the original slave process for a nodeExecList after we get the StartReq
 */
func slaveProc(r *RpcClientServer, inforpc *RpcClientServer, returnrpc *RpcClientServer) {
	// Make sure the root (default /tmp/xproc) exists
	os.Mkdir(*binRoot, 0700)
	// Do a private mount if necessary
	if *DoPrivateMount == true {
		doPrivateMount(*binRoot)
	}

	done := make(chan int, 0) // this is how we'll know the command is done

	// Receive a StartReq from the master/parent
	req := &StartReq{}
	if r.Recv("slaveProc", &req) != nil {
		log.Fatal("failed on receiving a start request")
	}
	Dprint(2, "slaveProc: req ", *req)

	// Establish a connection to the IO proxy
	n, c, err := fileTcpDial(req.Lserver)
	if err != nil {
		log.Print("tcpDial: ", err)
		return
	}

	// Run the program
	go runLocal(req, n, done)

	/* the child may end before we even get here, but since we still own this name 
	 * space, the files are still there. Now we set up an ioProxy and copy the StartReq
	 * and files to any children we may have.
	 */
	var nodesCopy string
	var l Listener
	var workerChan chan int
	var numWorkers int
	numWorkers = 0
	nodesCopy = req.Nodes
	slaveNodes, err := parseNodeList(nodesCopy)
	if err != nil {
		return
	}
	returnrpc.Send("send slaveNodes ", slaveNodes[0])
	var availableSlaves nodeExecList
	if inforpc.Recv("recv availableSlaves", &availableSlaves) != nil {
		return
	}
	Dprint(2, "receiveCmds: sendReq.Nodes: ", req.Nodes, " expands to ", slaveNodes)

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
	nnodes := sendCommandsToANodeSet(req, slaveNodes[0].Subnodes, *binRoot, availableSlaves.Nodes)
	Dprint(2, "Sent to ", nnodes, " nodes")
	// Wait for all the children to finish execution
	for numWorkers > 0 {
		worker := <-workerChan
		Dprint(2, worker, " returned, ", numWorkers, " workers left")
		numWorkers--
	}
	<-done // wait until our own instance has finished executing
	c.Close()
	n.Close()
	Dprint(2, "Exiting slaveProc")
}

/*
 * This function is used to run a program which has been specified in
 * a StartReq and sent to the slave.
 *
 * 'n' points to a connection to the ioProxy directly "above" us.
 */
func runLocal(req *StartReq, n *os.File, done chan int) {
	Dprint(2, "runLocal: dialed %v", n)
	f := []*os.File{n, n, n} // set up stdin/stdout/stderr for the program
	var pathbase = *binRoot
	execpath := pathbase + req.Path + req.Args[0]
	if req.LocalBin {
		execpath = req.Args[0]
	}
	Dprint(2, "run: execpath: ", execpath)
	Env := req.Env
	/* now build the LD_LIBRARY_PATH variable */
	ldLibPath := "LD_LIBRARY_PATH="
	for _, s := range req.LibList {
		ldLibPath = ldLibPath + *binRoot + req.Path + s + ":"
	}
	Env = append(Env, ldLibPath)
	Dprint(2, "run: Env ", Env)
	procattr := os.ProcAttr{Env: Env, Dir: pathbase + "/" + req.Cwd,
		Files: f}
	Dprint(2, "run: dir: ", pathbase+"/"+req.Cwd)
	p, err := os.StartProcess(execpath, req.Args, &procattr)
	if err != nil {
		log.Fatal("run: ", err)
		n.Write([]uint8(err.String()))
	} else {
		w, _ := p.Wait(0)
		Dprint(2, "run: process returned ", w.String())
	}
	done <- 1 // we're called as a goroutine, so notify that we're done
}
