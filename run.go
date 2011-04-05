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
	"log"
	"net"
)

func runLocal(req *StartReq) {
	n, err := fileTcpDial(req.Lserver)
	if err != nil {
		log.Fatal("tcpDial: ", err)
	}
	defer n.Close()
	Dprint(2, "runLocal: dialed %v", n)
	f := []*os.File{n, n, n}
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
	procattr := os.ProcAttr{Env: os.Environ(), Dir: pathbase + "/" + req.Cwd,
		Files: f}
	Dprint(2, "run: dir: ", pathbase + "/" + req.Cwd)
//	procattr := os.ProcAttr{Env: nil, Dir: "", Files: f}
	_, err = os.StartProcess(execpath, req.Args, &procattr)
	Dprint(2, "run: process exited")
	if err != nil {
		log.Fatal("run: ", err)
		n.Write([]uint8(err.String()))
	}
}

func runPeers(req *StartReq, nodes *nodeExecList) {
	var numWorkers int
	var numOtherNodes int
	var l Listener
	var workerChan chan int
	Dprint(4, "peers = %s", req.Peers)

	if req.Peers != nil || numOtherNodes > 0 {
		parentConn, err := net.Dial(req.Lfam, req.Lserver)
		if err != nil {
			log.Fatalf("run: ioproxy: ", err)
		}
		workerChan, l, err = ioProxy(defaultFam, loc.Ip()+":0", parentConn)
		if err != nil {
			log.Fatalf("run: ioproxy: ", err)
		}
		Dprint(2, "netwaiter locl.IP() ", loc.Ip(), " listener at ", l.Addr().String())
		req.Lfam = l.Addr().Network()
		req.Lserver = l.Addr().String()
	}
	if req.Peers != nil {
		Dprint(2, "run: Peers: ", req.Peers)
		/* this might be a test */
		switch {
		default:
			for _, p := range req.Peers {
				numWorkers += 2
				go func(w chan int) {
					cacheRelayFilesAndDelegateExec(req, *binRoot, p)
					w <- 1
				}(workerChan)
			}
		case req.PeerGroupSize > 0:
			/* this is quite inefficient but rarely used so I'm not that concerned */
			larg := newStartReq(req)
			server := ""
			server, larg.Peers = larg.Peers[0], larg.Peers[1:]
			Dprint(2, "run: chain to ", server, " chain workers: ", larg.Peers)
			numWorkers = 2
			go func(w chan int) {
				cacheRelayFilesAndDelegateExec(larg, *binRoot, server)
				w <- 1
			}(workerChan)
		}
	}
	/* note: sendCommands needs to be refactored, then we can consider using
	 * the bits. Which is true of this whole package, but that's life. Need a go func 
	 * here so as not to hang on hung nodes
	 */
	if numOtherNodes > 0 {
		numWorkers += numOtherNodes
		Dprint(2, "Send commands to ", nodes)
		for _, s := range nodes.Nodes {
			Dprint(2, "Send commands to ", s)
			go func(Server string) {
				Dprint(2, "Go func ", Server)
				nr := newStartReq(req)
				nr.Peers = nil
				nr.Nodes = nodes.Subnodes
				cacheRelayFilesAndDelegateExec(nr, *binRoot, Server)
				Dprintf(2, "cacheRelayFilesAndDelegateExec DONE\n")
			}(s)
		}
	}

	WaitAllChildren()
	Dprint(2, "All Children done, ", numWorkers, " workers left")
	for numWorkers > 0 {
		worker := <-workerChan
		Dprint(2, worker, " returned, ", numWorkers, " workers left")
		numWorkers--
	}
	Dprint(2, "Done")
	os.Exit(0)
}


