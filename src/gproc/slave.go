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
	"encoding/gob"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"
)

var id string

func runSlave() {
	/* some simple sanity checking */
	if *DoPrivateMount == true && os.Getuid() != 0 {
		log_error("Slave: Need to run as root for private mounts")
	}
	if *parent == "" {
		log_error("Slave: must set parent IP with -myParent switch")
	}
	if *myAddress == "" {
		log_error("Slave: must set myAddress IP with -myAddress switch")
	}
	if *myId == "" {
		log_error("Slave: must set myId with -myId switch")
	}

	/* at this point everything is right. So go forever. */
	for {
		startSlave()
		log_info("Slave returned; try a timeout")
		/* sleep for a random time that is between 10 and 60 seconds. random is necessary
		 * because we have seen self-synchronization in earlier work. 
		 */
		r := int64(rand.Intn(50) + 10)
		time.Sleep(time.Duration(r * int64(1<<30)))
	}
}

/* We will for now assume that addressing is symmetric, that is, if we Dial someone on
 * a certain address, that's the address they should Dial us on. This assumption has held
 * up well for quite some time. And, in fact, it makes no sense to do it any other way ...
 */
/* note that we're going to be able to merge master and slave fairly soon, now that they do almost the same things. */
func startSlave() {
	/* slight difference from master: we're ready when we start, since we run things */
	vitalData := &vitalData{HostReady: true, Id: *myId}
	masterAddr := *parent + ":" + *cmdPort
	log_info("dialing masterAddr ", masterAddr)
	master, err := Dial(*defaultFam, "", masterAddr)
	if err != nil {
		log_error("startSlave: dialing:", err)
		return
	}

	/* vitalData -- what we're doing here is assembling information for our parent. 
	 * we have to tell our parent what port we look for process startup commands on, 
	 * the address of our side of the Dial connection, and, due to a limitation in the Unix
	 * kernels going back a long time, we might as well tell the master its own address for
	 * the socket, since *the master can't get it*. True! 
	 */
	addr := strings.SplitN(master.LocalAddr().String(), ":", -1)
	peerAddr := addr[0] + ":0"

	laddr, _ := net.ResolveTCPAddr("tcp4", peerAddr)
	netl, err := net.ListenTCP(*defaultFam, laddr)
	if err != nil {
		log_error("startSlave: ", err)
		return
	}
	vitalData.ServerAddr = netl.Addr().String()
	vitalData.HostAddr = master.LocalAddr().String()
	vitalData.ParentAddr = master.RemoteAddr().String()
	r := NewRpcClientServer(master, *binRoot)
	initSlave(r, vitalData)
	go registerSlaves()
	/* wow. This used to be much smaller and needs to be redone. */
	go func() {
		for {
			// Wait for a connection from the master
			c, err := netl.AcceptTCP()
			if err != nil {
				log_info("problem in netl.Accept()")
				return
			}
			log_info("Received connection from: ", c.RemoteAddr())

			// start a new process, give it 'c' as stdin.
			connFile, _ := c.File()                              // the new process will read a StartReq from connFile
			readp, writep, _ := os.Pipe()                        // we'll send a list of slaves over this
			readp2, writep2, _ := os.Pipe()                      // the child will send a list of nodes and ask for a list of slaves
			f := []*os.File{connFile, readp, os.Stderr, writep2} // we can't use Stderr because the child wants to write to it
			cwd, _ := os.Getwd()
			procattr := os.ProcAttr{Env: nil, Dir: cwd, Files: f}
			argv := []string{
				"gproc",
				fmt.Sprintf("-debug=%v", *Extra_debug),
				fmt.Sprintf("-p=%v", *DoPrivateMount),
				fmt.Sprintf("-binRoot=%v", *binRoot),
				fmt.Sprintf("-parent=%v", *parent),
				"-prefix=" + id,
				"R", // "R" = run a program
			}
			// Start the new process
			p, err := os.StartProcess(*gprocBin, argv, &procattr)
			if err != nil {
				log_error("startSlave: ", err)
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

				w, _ := p.Wait() // Wait until the child process is finished. We need to do things sorta synchronously
				log_info("startSlave: process returned ", w.String())
			}
			c.Close()
			writep.Close()
			readp2.Close()
		}
	}()

	// This read doesn't really matter, the important thing is that it will fail when the master goes away
	foo := &StartReq{}
	r.Recv("slaveProc done", &foo)
	/* instead of returning here, just exit. This is because the master went away, so we want to destroy the whole tree */
	os.Exit(0)
}

func initSlave(r *RpcClientServer, v *vitalData) {
	log_info("initSlave: ", v)
	r.Send("startSlave", *v)
	resp := &SlaveResp{}
	if r.Recv("startSlave", &resp) != nil {
		log_error("Can't start slave")
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
		log_error("failed on receiving a start request")
	}
	log_info("slaveProc: req ", *req)

	// Establish a connection to the IO proxy
	n, c, err := fileTcpDial(req.Lserver)
	if err != nil {
		log_info("tcpDial: ", err)
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
	log_info("receiveCmds: sendReq.Nodes: ", req.Nodes, " expands to ", slaveNodes)

	if len(availableSlaves.Nodes) > 0 {
		workerChan, l, err = ioProxy(*defaultFam, *myAddress+":0", c)
		if err != nil {
			log_error("slave: ioproxy: ", err)
		}
		log_info("netwaiter locl.Ip() ", *myAddress, " listener at ", l.Addr().String())
		req.Lfam = l.Addr().Network()
		req.Lserver = l.Addr().String()

		for _, _ = range availableSlaves.Nodes {
			numWorkers += 1
		}
	}
	nnodes := sendCommandsToANodeSet(req, slaveNodes[0].Subnodes, *binRoot, availableSlaves.Nodes)
	log_info("Sent to ", nnodes, " nodes")
	// Wait for all the children to finish execution
	for numWorkers > 0 {
		worker := <-workerChan
		log_info(worker, " returned, ", numWorkers, " workers left")
		numWorkers--
	}
	<-done // wait until our own instance has finished executing
	c.Close()
	n.Close()
	log_info("Exiting slaveProc")
}

/*
 * This function is used to run a program which has been specified in
 * a StartReq and sent to the slave.
 *
 * 'n' points to a connection to the ioProxy directly "above" us.
 */
func runLocal(req *StartReq, n *os.File, done chan int) {
	log_info("runLocal: dialed %v", n)
	f := []*os.File{n, n, n} // set up stdin/stdout/stderr for the program
	var pathbase = *binRoot
	execpath := pathbase + req.Path + req.Args[0]
	if req.LocalBin {
		execpath = req.Args[0]
	}
	log_info("run: execpath: ", execpath)
	Env := req.Env
	/* now build the LD_LIBRARY_PATH variable */
	ldLibPath := "LD_LIBRARY_PATH="
	for _, s := range req.LibList {
		ldLibPath = ldLibPath + *binRoot + req.Path + s + ":"
	}
	Env = append(Env, ldLibPath)
	log_info("run: Env ", Env)
	procattr := os.ProcAttr{Env: Env, Dir: pathbase + "/" + req.Cwd,
		Files: f}
	log_info("run: dir: ", pathbase+"/"+req.Cwd)
	p, err := os.StartProcess(execpath, req.Args, &procattr)
	if err != nil {
		log_error("run: ", err)
		n.Write([]uint8(err.Error()))
	} else {
		w, _ := p.Wait()
		log_info("run: process returned ", w.String())
	}
	done <- 1 // we're called as a goroutine, so notify that we're done
}
