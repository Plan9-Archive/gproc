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
		cmds[i].Name = s
		cmds[i].FullPath = root + s
		fi, _ := os.Stat(root + s)
		cmds[i].Fi = fi
	}
	return cmds
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
	resp.Msg = string(arg.Msg)
	return nil
}

func Debug(arg *SetDebugLevel, resp *SetDebugLevel) os.Error {
	resp.level = *DebugLevel
	*DebugLevel = arg.level
	return nil
}
