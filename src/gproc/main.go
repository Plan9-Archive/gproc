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
	"bitbucket.org/floren/gproc/src/forth"
	"encoding/gob"
	"flag"
	"fmt"
	"log"
	"os"
)

func usage() {
	fmt.Fprint(os.Stderr, "usage: gproc m\n")
	fmt.Fprint(os.Stderr, "usage: gproc s\n")
	fmt.Fprint(os.Stderr, "usage: gproc e <nodes> <command>\n")
	fmt.Fprint(os.Stderr, "usage: gproc i [i ...] goes one level deeper for each i\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	Logfile        = "/tmp/log"
	prefix         = flag.String("prefix", "", "logging prefix")
	localbin       = flag.Bool("localbin", false, "execute local files")
	DoPrivateMount = flag.Bool("p", true, "Do a private mount")
	Extra_debug    = flag.Bool("debug", false, "enable extra debug info")
	/* this one gets me a zero-length string if not set. Phooey. */
	filesToTakeAlong = flag.String("f", "", "comma-seperated list of files/directories to take along")
	root             = flag.String("r", "", "root for finding binaries")
	libs             = flag.String("L", "/lib:/usr/lib", "library path")
	binRoot          = flag.String("binRoot", "/tmp/xproc", "Where to put binaries and libraries")
	defaultMasterUDS = flag.String("defaultMasterUDS", "/tmp/g", "Default Master Unix Domain Socket")
	ioProxyPort      = flag.String("iopp", "0", "io proxy port")
	cmdPort          = flag.String("cmdport", "6666", "command port")
	defaultFam       = flag.String("fam", "tcp4", "network type")
	gprocBin         = flag.String("gprocBin", "gproc", "name of gproc binary")
	/* required in the command line */
	parent    = flag.String("myParent", "hostname", "parent for some configurations")
	myAddress = flag.String("myAddress", "hostname", "Required set to my address")
	myId      = flag.String("myId", "0", "Required -- tell slaves their id")
	/* these are not switches */
	role            = "client"
	myListenAddress string
)

/* 
 * some examples: -myId 'hostname base 7 / hostname base 7 % dup ifelse'
 * which is the same as taking, given a hostname of sb12, 
 * i = atoi(&sb[2]); return (i %7 == 0) ? i % 7 : i / 7
 * for the parent: 'hostname base 7 roundup sb strcat 10.0.0.253 hostname base 7 % ifelse'
 * returns same of i = atoi(&sb[2]); return (i %7 == 0) ? 10.0.0.253 : strcat(sb[0:1, (((i+6)/7)*7))
 * You can just put a simple string in for the argument and it will push and pop it
 *  ./gproc_linux_arm -parent='hostname base 7 roundup sb strcat 10.1.1.1 hostname base 7 % ifelse' -myId='hostname base 7 % 1  + hostname base 7 / hostname base 7 % ifelse' -myAddress=hostname s
 * is used for strongbox, where there is a single root, 49 level 1s defined by name %7 == 0, and 
 */
func main() {
	var err error
	flag.Usage = usage
	flag.Parse()
	interp := forth.New()
	*myId, err = forth.Eval(interp, *myId)
	if err != nil {
		log_error(err)
	}
	*myAddress, err = forth.Eval(interp, *myAddress)
	if err != nil {
		log_error(err)
	}
	*parent, err = forth.Eval(interp, *parent)
	if err != nil {
		log_error(err)
	}
	log_info("My id is %v; parent %v; address %v\n", *myId, *parent, *myAddress)
	myListenAddress = *myAddress + ":" + *cmdPort
	log.SetPrefix("newgproc " + *prefix + ": ")
	log_info("starting:", os.Args)

	switch flag.Arg(0) {
	/* traditional bproc master, commands over unix domain socket */
	case "MASTER", "master", "m":
		if len(flag.Args()) > 1 {
			flag.Usage()
		}
		startMaster()
	case "WORKER", "worker", "s":
		/* traditional slave; connect to master, await instructions */
		if len(flag.Args()) != 1 {
			flag.Usage()
		}
		runSlave()
	case "EXEC", "exec", "e":
		/* Issuing a command to run on the slaves */
		if len(flag.Args()) < 3 {
			flag.Usage()
		}
		startExecution(*defaultMasterUDS, *defaultFam, *ioProxyPort, flag.Arg(1), flag.Args()[2:])
	case "INFO", "info", "i":
		/* Get info about the available nodes */
		if len(flag.Args()) > 1 {
			flag.Usage()
		}
		info := getInfo(*defaultMasterUDS, flag.Arg(1))
		fmt.Print("Nodes:\n", info)
		/* not yet
		case "EXCEPT", "except", "x":
		loc.Init("init")
		exceptOK := except(*defaultMasterUDS, flag.Args()[1:])
		fmt.Print(exceptOK)
		*/
	case "R":
		/* This is for executing a program from the slave */
		slaveProc(NewRpcClientServer(os.Stdin, *binRoot), &RpcClientServer{E: gob.NewEncoder(os.Stdout), D: gob.NewDecoder(os.Stdout)}, &RpcClientServer{E: gob.NewEncoder(os.NewFile(3, "pipe")), D: gob.NewDecoder(os.NewFile(3, "pipe"))})
	default:
		flag.Usage()
	}
}
