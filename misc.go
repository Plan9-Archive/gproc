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
	"fmt"
	"log"
	"net"
	"os"
	"bitbucket.org/npe/ldd"
)

/* let's be nice and do an Ldd on each file. That's helpful to people. Later. */
func buildcmds(file, root, libs string) []*cmdToExec {
	e, _ := ldd.Ldd(file, root, libs)
	/* now we have a list of file names. From this, we create the in-memory
	 * packed set of files/symlinks/directory descriptions. We also need to track
	 * what weve made and might have made earlier, to avoid duplicates.
	 */
	cmds := make([]*cmdToExec, len(e))
	for i, s := range e {
		cmds[i].name = s
		cmds[i].fullPath = root + s
		fi, _ := os.Stat(root + s)
		cmds[i].fi = fi
	}
	return cmds
}

func netwaiter(fam, server string, numWorkers int, c net.Conn) (chan int, Listener, os.Error) {
	workerChan := make(chan int, numWorkers)
	l, err := Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %s\n", err)
		return nil, l, err
	}

	go func() {
		for ; numWorkers > 0; numWorkers-- {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("%s\n", err)
				continue
			}
			go netrelay(conn, workerChan, c)
		}
	}()
	return workerChan, l, nil
}

func netrelay(c net.Conn, workerChan chan int, client net.Conn) {
	data := make([]byte, 1024)
	for {
		n, _ := c.Read(data)
		if n <= 0 {
			break
		}
		amt, err := client.Write(data)
		if amt <= 0 {
			log.Printf("Write failed: amt %d, err %v\n", amt, err)
			break
		}
		if err != nil {
			log.Printf("Write failed: %v\n", err)
			break
		}
	}
	workerChan <- 1
}

func readitin(s, root string) ([]byte, os.FileInfo, os.Error) {
	fi, _ := os.Stat(root + s)
	f, _ := os.Open(s, os.O_RDONLY, 0)
	bytes := make([]byte, fi.Size)
	f.Read(bytes)
	return bytes, *fi, nil
}

type Arg struct {
	Msg []byte
}

func Ping(arg *Arg, resp *Resp) os.Error {
	resp.Msg = arg.Msg
	return nil
}

func Debug(arg *SetDebugLevel, resp *SetDebugLevel) os.Error {
	resp.level = *DebugLevel
	*DebugLevel = arg.level
	return nil
}
