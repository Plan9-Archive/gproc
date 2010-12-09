package main

import (
	"os"
	"log"
	"io"
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
	var numWorkers int
	var pathbase = *binRoot
	log.SetPrefix("run "+*prefix+": ")
	r := NewRpcClientServer(os.Stdin)
	var req StartReq
	r.Recv("run", &req)
	/* make sure the directory exists and then do the private name space mount */

	Dprintf(3, "run: req is %v\n", req)
	os.Mkdir(pathbase, 0700)
	if *DoPrivateMount == true {
		doPrivateMount(pathbase)
	}
	for _, c := range req.cmds {
		Dprintf(2, "run: Localbin %v cmd %v: ", req.LocalBin, c)
		Dprintf(2, "%s\n", c.name)
		_, err := writeStreamIntoFile(os.Stdin, c)
		if err != nil {
			log.Exit("run: writeStreamIntoFile: ", err)
		}
	}
	Dprintf(2, "run: connect to %v\n", req.Lserver)
	n := fileTcpDial(req.Lserver)
	f := []*os.File{n, n, n}
	execpath := pathbase + req.Args[0]
	if req.LocalBin {
		execpath = req.Args[0]
	}
	Dprint(2,"run: execpath: ",execpath)
	_, err := os.ForkExec(execpath, req.Args, req.Env, pathbase, f)
	n.Close()

	if err != nil {
		log.Exit("run: ", err)
	}
	
	if req.Peers != nil {
		Dprint(2,"run: Peers: ",req.Peers)
		/* this might be a test */
		if req.chainWorkers {
			/* this is quite inefficient but rarely used so I'm not that concerned */
			larg := newStartReq(&req)
			p := larg.Peers[0]
			newPeers := make([]string, len(larg.Peers)-1)
			copy(newPeers, larg.Peers[1:])
			larg. Peers = newPeers
			Dprint(2,"run: chain to ", p, " chain workers: ", newPeers)
			workerChan = make(chan int, 1)
			numWorkers = 1
			go func (w chan int) {
				cacheRelayFilesAndDelegateExec(larg, *binRoot, p)
				w <- 1
			}(workerChan)
		} else {
			numWorkers = len(req.Peers)
			workerChan = make(chan int, numWorkers)
			for _, p := range req.Peers {
				go func (w chan int) {
					cacheRelayFilesAndDelegateExec(&req, *binRoot, p)
					w <- 1
				}(workerChan)

			}
		}
	}

	WaitAllChildren()
	for numWorkers > 0 {
		<-workerChan
		numWorkers--
	}
	os.Exit(0)
}

func fileTcpDial(server string) *os.File {
	// percolates down from startExecution
	sock := tcpSockDial(server)
	Dprintf(2, "run: connected to %v\n", server)
	if sock < 0 {
		log.Exitf("fileTcpDial: connect to %s failed", server)
	}
	return os.NewFile(sock, "child_process_socket")
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
	outputFile := path.Join(*binRoot, c.name)
	fi := c.fi
	Dprintf(2, "writeStreamIntoFile: ", outputFile," ", c)
	switch  {
	case fi.IsDirectory():
		Dprint(5, "writeStreamIntoFile: is dir ", fi.Name)
		err = os.MkdirAll(outputFile, fi.Mode&0777)
		if err != nil {
			err = os.Chown(outputFile, fi.Uid, fi.Gid)
		}
	case fi.IsSymlink():
		Dprint(5, "writeStreamIntoFile: is link")
		dir, _ := path.Split(outputFile)
		_, err = os.Lstat(dir)
		if err != nil {
			os.MkdirAll(dir, 0777)
			err = nil
		}
		err = os.Symlink(outputFile, *binRoot+ c.fullPath)
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
		Dprint(5, "writeStreamIntoFile: copying ",fi.Name, " ", fi.Size)
		n, err = io.Copyn(f, stream, fi.Size)
		Dprint(5, "writeStreamIntoFile: copied ",fi.Name, " ",n)
		if err != nil {
			log.Exit("writeStreamIntoFile: copyn: ",err)
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
