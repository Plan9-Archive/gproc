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

	if *DebugLevel > 3 {
		log.Printf("arg is %v\n", arg)
	}
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
		if *DebugLevel > 2 {
			log.Printf("Localbin %v cmd %v:", arg.LocalBin, s)
			log.Printf("%s\n", s.name)
		}
		_, err := writeitout(os.Stdin, s.name, s.fi)
		if err != nil {
			break
		}
	}
	if *DebugLevel > 2 {
		log.Printf("Connect to %v\n", arg.Lserver)
	}

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
