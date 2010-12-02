package main

import (
	"os"
	"log"
	"gob"
	"syscall"
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
 * each peer and mexecclient for the children.
 */
func run() {
	var arg StartArg
	var pathbase = "/tmp/xproc"
	d := gob.NewDecoder(os.Stdin)
	d.Decode(&arg)
	/* make sure the directory exists and then do the private name space mount */

	Dprintf(3, "arg is %v\n", arg)
	os.Mkdir(pathbase, 0700)
	if *DoPrivateMount == true {
		unshare()
		_ = unmount(pathbase)
		syscallerr := privatemount(pathbase)
		if syscallerr != 0 {
			log.Printf("Mount failed", syscallerr, "\n")
			os.Exit(1)
		}
	}

	for _, s := range arg.cmds {
		Dprintf(2, "Localbin %v cmd %v:", arg.LocalBin, s)
		Dprintf(2, "%s\n", s.name)
		_, err := writeitout(os.Stdin, s.name, s.fi)
		if err != nil {
			break
		}
	}
	Dprintf(2, "Connect to %v\n", arg.Lserver)

	sock := connect(arg.Lserver)

	if sock < 0 {
		os.Exit(1)
	}
	n := os.NewFile(sock, "child_process_socket")
	f := []*os.File{n, n, n}
	execpath := pathbase + arg.Args[0]
	if arg.LocalBin {
		execpath = arg.Args[0]
	}
	log.Println("execpath",execpath)
	_, err := os.ForkExec(execpath, arg.Args, arg.Env, pathbase, f)
	n.Close()
	if err == nil {
		go func() {
			var status syscall.WaitStatus
			for pid, err := syscall.Wait4(-1, &status, 0, nil); err > 0; pid, err = syscall.Wait4(-1, &status, 0, nil) {
				log.Printf("wait4 returns pid %v status %v\n", pid, status)
			}
		}()
	} else {
		if *DebugLevel > 2 {
			log.Printf("ForkExec failed: %s\n", err)
		}
	}
	os.Exit(1)
}


func writeitout(in *os.File, s string, fi os.FileInfo) (int, os.Error) {
	var err os.Error
	var filelen int = 0
	out := "/tmp/xproc" + s
	if *DebugLevel > 2 {
		log.Printf("write out  %s, %v %v\n", out, fi, fi.Mode)
	}
	switch fi.Mode & syscall.S_IFMT {
	case syscall.S_IFDIR:
		err = os.Mkdir(out, fi.Mode&0777)
		if err != nil {
			err = os.Chown(out, fi.Uid, fi.Gid)
		}
	case syscall.S_IFLNK:
		err = os.Symlink(out, "/tmp/xproc"+fi.Name)
	case syscall.S_IFREG:
		f, err := os.Open(out, os.O_RDWR|os.O_CREAT, 0777)
		if err != nil {
			return -1, err
		}
		defer f.Close()
		b := make([]byte, 8192)
		for i := int64(0); i < fi.Size; {
			var amt int = int(fi.Size - i)
			if amt > len(b) {
				amt = len(b)
			}
			amt, _ = in.Read(b[0:amt])
			amt, err = f.Write(b[0:amt])
			if err != nil {
				return -1, err
			}
			i += int64(amt)
			if *DebugLevel > 5 {
				log.Printf("Processed %d of %d\n", i, fi.Size)
			}
		}
		if *DebugLevel > 5 {
			log.Printf("Done %v\n", out)
		}
		if err != nil {
			err = os.Chown(out, fi.Uid, fi.Gid)
		}
	default:
		return -1, nil
	}

	if *DebugLevel > 2 {
		log.Printf("Finished %v\n", out)
	}
	return filelen, nil
}
