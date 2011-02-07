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
	"log"
	"io"
	"net"
	"path"
)


/* started by gproc. Data comes in on stdin. We create the
 * whole file tree in a private name space -- this is
 * to keep the process image from growing too big.
 * we almost certainly exec it. Then we send all those
 * files right back out again to other nodes if needed
 * (later).
 * We always make and mount binRoot, and chdir to it, so the
 * programs have a safe place to stash files that might go away after
 * all is done.
 * Due to memory footprint issues, we really can not have both the
 * files and a copy of the data in memory. (the files are in ram too).
 * So this function is responsible for issuing the commands to our
 * peerlist as well as to any subnodes. We run a goroutine for
 * each peer and sendCommandsAndWriteOutFiles for the children.
 */
func run() {
	var workerChan chan int
	var l Listener
	var numWorkers int
	var pathbase = *binRoot
	log.SetPrefix("run " + *prefix + ": ")
	r := NewRpcClientServer(os.Stdin)
	var req StartReq
	r.Recv("run", &req)
	var nodeExecList nodeExecList
	r.Recv("nodeExecList", &nodeExecList)

	numOtherNodes := len(nodeExecList.Nodes)
	Dprintf(3, "run: req is %v; nodeExeclist is %v (len %d)\n", req, nodeExecList, numOtherNodes)

	/* make sure the directory exists and then do the private name space mount */
	os.Mkdir(pathbase, 0700)
	if *DoPrivateMount == true {
		doPrivateMount(pathbase)
	}

	for _, c := range req.Cmds {
		Dprintf(2, "run: Localbin %v cmd %v: ", req.LocalBin, c)
		Dprintf(2, "%s\n", c.Name)
		_, err := writeStreamIntoFile(os.Stdin, c)
		if err != nil {
			log.Fatal("run: writeStreamIntoFile: ", err)
		}
	}

	Dprintf(2, "run: connect to %v\n", req.Lserver)
	n, err := fileTcpDial(req.Lserver) // connect to the ioproxy.
	if err != nil {
		log.Fatal("tcpDial: ", err)
	}
	defer n.Close()
	f := []*os.File{n, n, n}
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
	_, err = os.ForkExec(execpath, req.Args, Env, pathbase + "/" + req.Cwd, f)

	if err != nil {
		log.Fatal("run: ", err)
		n.Write([]uint8(err.String()))
	}

	if req.Peers != nil || numOtherNodes > 0 {
		parentConn, err := net.Dial(req.Lfam, "", req.Lserver)
		if err != nil {
			log.Fatalf("run: ioproxy: ", err)
		}
		workerChan, l, err = netwaiter(defaultFam, loc.Ip()+":0", len(req.Peers)+numOtherNodes, parentConn)
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
				go func(w chan int) {
					cacheRelayFilesAndDelegateExec(&req, *binRoot, p)
					w <- 1
				}(workerChan)
			}
		case req.PeerGroupSize > 0:
			/* this is quite inefficient but rarely used so I'm not that concerned */
			larg := newStartReq(&req)
			server := ""
			server, larg.Peers = larg.Peers[0], larg.Peers[1:]
			Dprint(2, "run: chain to ", server, " chain workers: ", larg.Peers)
			numWorkers = 1
			workerChan = make(chan int, numWorkers)
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
		Dprint(2, "Send commands to ", nodeExecList)
		for _, s := range nodeExecList.Nodes {
			Dprint(2, "Send commands to ", s)
			go func(Server string) {
				Dprint(2, "Go func ", Server)
				nr := newStartReq(&req)
				nr.Peers = nil
				nr.Nodes = nodeExecList.Subnodes
				cacheRelayFilesAndDelegateExec(nr, *binRoot, Server)
				Dprintf(2, "cacheRelayFilesAndDelegateExec DONE\n")
			}(s)
		}
	}

	WaitAllChildren()
	for numWorkers > 0 {
		<-workerChan
		numWorkers--
	}
	os.Exit(0)
}

func fileTcpDial(server string) (*os.File, os.Error) {
	var laddr net.TCPAddr
	raddr, err := net.ResolveTCPAddr(server)
	if err != nil {
		return nil, err
	}
	c, err := net.DialTCP(defaultFam, &laddr, raddr)
	if err != nil {
		return nil, err
	}
	f, err := c.File()
	if err != nil {
		c.Close()
		return nil, err
	}

	return f, nil
}


func doPrivateMount(pathbase string) {
	unshare()
	_ = unmount(pathbase)
	syscallerr := privatemount(pathbase)
	if syscallerr != 0 {
		log.Print("Mount failed ", syscallerr)
		os.Exit(1)
	}
}

func writeStreamIntoFile(stream *os.File, c *cmdToExec) (n int64, err os.Error) {
	outputFile := path.Join(*binRoot, c.Name)
	fi := c.Fi
	Dprintf(2, "writeStreamIntoFile: ", outputFile, " ", c)
	switch {
	case fi.IsDirectory():
		Dprint(5, "writeStreamIntoFile: is dir ", fi.Name)
		err = os.MkdirAll(outputFile, fi.Mode&0777)
		if err != nil {
			err = os.Chown(outputFile, fi.Uid, fi.Gid)
		}
	case fi.IsSymlink():
		Dprint(5, "writeStreamIntoFile: is link")
		// c.Fullpath is the symlink target
		dir, _ := path.Split(outputFile)
		_, err = os.Lstat(dir)
		if err != nil {
			os.MkdirAll(dir, 0777)
			err = nil
		}

		if c.FullPath[0] == '/' {
			// if the link is absolute we glom on our root prefix
			c.FullPath = *binRoot + c.FullPath
		}
		err = os.Symlink(c.FullPath, outputFile)
		// kinda a weird bug. When not using provate mounts
		// and a symlink already exists it has err ="file exists"
		// but is not == to os.EEXIST.. need to check gocode for actual return
		// its probably not exported either...
	case fi.IsRegular():
		Dprint(5, "writeStreamIntoFile: is regular file")
		dir, _ := path.Split(outputFile)
		_, err = os.Lstat(dir)
		if err != nil {
			os.MkdirAll(dir, 0777)
			err = nil
		}
		f, err := os.Open(outputFile, os.O_RDWR|os.O_CREAT, 0777)
		if err != nil {
			return
		}
		defer f.Close()
		Dprint(5, "writeStreamIntoFile: copying ", fi.Name, " ", fi.Size)
		n, err = io.Copyn(f, stream, fi.Size)
		Dprint(5, "writeStreamIntoFile: copied ", fi.Name, " ", n)
		if err != nil {
			log.Fatal("writeStreamIntoFile: copyn: ", err)
		}
		if err != nil {
			err = os.Chown(outputFile, fi.Uid, fi.Gid)
		}
	default:
		return
	}

	Dprint(2, "writeStreamIntoFile: finished ", outputFile)
	return
}
