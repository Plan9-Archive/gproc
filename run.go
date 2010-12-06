package main

import (
	"os"
	"log"
	"syscall"
	"io"
	"path"
)


/* started by gproc. Data comes in on stdin. We create the
 * whole file tree in a private name space -- this is
 * to keep the process image from growing too big.
 * we almost certainly exec it. Then we send all those
 * files right back out again to other nodes if needed
 * (later).
 * We always make and mount /tmp/xproc, and chdir to it, so the
 * programs have a safe place to stash files that might go away after
 * all is done.
 * Due to memory footprint issues, we really can not have both the
 * files and a copy of the data in memory. (the files are in ram too).
 * So this function is responsible for issuing the commands to our
 * peerlist as well as to any subnodes. We run a goroutine for
 * each peer and sendCommandsAndWriteOutFiles for the children.
 */



func run() {
	var arg StartReq
	var pathbase = "/tmp/xproc"
	log.SetPrefix("run "+*prefix+": ")
	r := NewRpcClientServer(os.Stdin)
	r.Recv("run", &arg)
	/* make sure the directory exists and then do the private name space mount */

	Dprintf(3, "run: arg is %v\n", arg)
	os.Mkdir(pathbase, 0700)
	if *DoPrivateMount == true {
		doPrivateMount(pathbase)
	}
	for _, s := range arg.cmds {
		Dprintf(2, "run: Localbin %v cmd %v:", arg.LocalBin, s)
		Dprintf(2, "%s\n", s.name)
		_, err := writeStreamIntoFile(os.Stdin, s.name, s.fi)
		if err != nil {
			log.Exit("run: writeStreamIntoFile: ", err)
		}
	}
	Dprintf(2, "run: connect to %v\n", arg.Lserver)
	n := fileTcpDial(arg.Lserver)
	f := []*os.File{n, n, n}
	execpath := pathbase + arg.Args[0]
	if arg.LocalBin {
		execpath = arg.Args[0]
	}
	Dprint(2,"run: execpath: ",execpath)
	_, err := os.ForkExec(execpath, arg.Args, arg.Env, pathbase, f)
	n.Close()
	if err == nil {
		WaitAllChildren()
	} else {
		log.Exit("run: ", err)
	}
	os.Exit(0)
}

func fileTcpDial(server string) *os.File {
	// percolates down from startExecution
	sock := tcpSockDial(server)
	Dprintf(2, "run: connected to %v\n", server)
	if sock < 0 {
		log.Exit("fileTcpDial: connect to %s failed", server)
	}
	return os.NewFile(sock, "child_process_socket")
}


func doPrivateMount(pathbase string) {
	unshare()
	_ = unmount(pathbase)
	syscallerr := privatemount(pathbase)
	if syscallerr != 0 {
		log.Printf("Mount failed", syscallerr, "\n")
		os.Exit(1)
	}
}

func writeStreamIntoFile(stream *os.File, s string, fi *os.FileInfo) (n int64, err os.Error) {
	out := "/tmp/xproc" + s
	Dprintf(2, "writeStreamIntoFile:  %s, %v %v\n", out, fi, fi.Mode)
	switch fi.Mode & syscall.S_IFMT {
	case syscall.S_IFDIR:
		Dprint(5, "writeStreamIntoFile: is dir")
		err = os.Mkdir(out, fi.Mode&0777)
		if err != nil {
			err = os.Chown(out, fi.Uid, fi.Gid)
		}
	case syscall.S_IFLNK:
		Dprint(5, "writeStreamIntoFile: is link")
		
		err = os.Symlink(out, "/tmp/xproc/"+fi.Name)
	case syscall.S_IFREG:
		Dprint(5, "writeStreamIntoFile: is regular file")
		dir, _ := path.Split(out)
		_, err = os.Lstat(dir)
		if err != nil {
			os.Mkdir(dir, 0777)
			err = nil
		}
		f, err := os.Open(out, os.O_RDWR|os.O_CREAT, 0777)
		if err != nil {
			return
		}
		defer f.Close()
		Dprint(5, "writeStreamIntoFile: copying ",fi.Size)
		n, err = io.Copyn(f, stream, fi.Size)
//		n, err = io.Copy(f, stream)
		Dprint(5, "writeStreamIntoFile: copied ",n)
		if err != nil {
			log.Exit("writeStreamIntoFile: copyn: ",err)
		}
		if err != nil {
			err = os.Chown(out, fi.Uid, fi.Gid)
		}
	default:
		return
	}

	Dprint(2, "writeStreamIntoFile: finished ", out)
	return
}
