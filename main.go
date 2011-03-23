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
	"log"
	"os"
	"rpc"
	"fmt"
	"strconv"
	"flag"
	"json"
	"io/ioutil"
)

const (
	srvAddr = "/tmp/srvaddr"
)

type noderange struct {
	Base int
	Ip   string
}

type gpconfig struct {
	Noderanges []noderange
}

func usage() {
	fmt.Fprint(os.Stderr, "usage: gproc m\n")
	fmt.Fprint(os.Stderr, "usage: gproc s <family> <address> <server address>\n")
	fmt.Fprint(os.Stderr, "usage: gproc e <server address> <fam> <address> <nodes> <command>\n")
	fmt.Fprint(os.Stderr, "usage: gproc R\n")
	fmt.Fprint(os.Stderr, "usage: gproc i [i ...] goes one level deeper for each i\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	Logfile        = "/tmp/log"
	prefix         = flag.String("prefix", "", "logging prefix")
	localbin       = flag.Bool("localbin", false, "execute local files")
	DoPrivateMount = flag.Bool("p", true, "Do a private mount")
	DebugLevel     = flag.Int("debug", 0, "debug level")
	/* this one gets me a zero-length string if not set. Phooey. */
	filesToTakeAlong = flag.String("f", "", "comma-seperated list of files/directories to take along")
	root             = flag.String("r", "", "root for finding binaries")
	libs             = flag.String("L", "/lib:/usr/lib", "library path")
	peerGroupSize    = flag.Int("npeers", 0, "number of peers to delegate to")
	binRoot          = flag.String("binRoot", "/tmp/xproc", "Where to put binaries and libraries")
	defaultMasterUDS = flag.String("defaultMasterUDS", "/tmp/g", "Default Master Unix Domain Socket")
	locale           = flag.String("locale", "local", "Your locale -- jaguar, strongbox, etc. defaults to local -- i.e. all daemons on same machine")
	loc              Locale
	ioProxyPort      = flag.String("iopp", "0", "io proxy port")
	parent           = flag.String("parent", "", "parent for some configurations")
	cmdPort		= flag.String("cmdPort", "6666", "command port")
	/* these are not switches */
	role = "client"
	/* these are determined by your local, and these values are "reasonable defaults" */
	/* they are intended to be modified as needed by localInit */
	defaultFam = "tcp4" /* arguably you might make this an option but it's kind of useless to do so */
	/* cmdPort    = "0" */
	/* covering for issues in Go libraries */
	/* net.LookUpHost fails if there is no DNS -- an incorrect behavior. On some locales (strongbox)
	* you can load this up with cn hostnames. On Jaguar, may be impractical. 
	 */
)

func main() {
	var err os.Error
	flag.Usage = usage
	flag.Parse()
	log.SetPrefix("newgproc " + *prefix + ": ")
	//setupLog()
	//config := getConfig()
	Dprintln(2, "starting:", os.Args, "debuglevel", *DebugLevel)

	loc, err = newLocale(*locale)
	if err != nil {
		log.Fatal(err)
	}
	if loc == nil {
		log.Fatal("no locale")
	}
	switch flag.Arg(0) {
	/* traditional bproc master, commands over unix domain socket */
	case "DEBUG", "debug", "d":
		SetDebugLevelRPC(flag.Arg(1), flag.Arg(2), flag.Arg(3))
	case "MASTER", "master", "m":
		if len(flag.Args()) > 1 {
			flag.Usage()
		}
		loc.Init("master")
		startMaster(*defaultMasterUDS, loc)
	case "WORKER", "worker", "s":
		/* traditional slave; connect to master, await instructions */
		if len(flag.Args()) != 1 {
			flag.Usage()
		}
		loc.Init("slave")
		startSlave(defaultFam, loc.ParentAddr(), loc)
	case "EXEC", "exec", "e":
		if len(flag.Args()) < 3 {
			flag.Usage()
		}
		loc.Init("client")
		startExecution(*defaultMasterUDS, defaultFam, *ioProxyPort, flag.Arg(1), flag.Args()[2:])
	case "INFO", "info", "i":
		if len(flag.Args()) > 1 {
			flag.Usage()
		}
		loc.Init("init")
		info := getInfo(*defaultMasterUDS, flag.Arg(1))
		fmt.Print("Nodes:\n", info)
	case "EXCEPT", "except", "x":
		loc.Init("init")
		exceptOK := except(*defaultMasterUDS, flag.Args()[1:])
		fmt.Print(exceptOK)
	case "RUN", "run", "R":
		loc.Init("run")
		run()
	default:
		flag.Usage()
	}
}

func setupLog() {
	logfile, err := os.Open(Logfile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal("No log file", err)
	}

	log.SetOutput(logfile)

	log.Printf("DoPrivateMount: %v\n", DoPrivateMount)
}

func SetDebugLevelRPC(fam, server, newlevel string) {
	var ans SetDebugLevel
	level, err := strconv.Atoi(newlevel)
	if err != nil {
		log.Fatal("bad level:", err)
	}

	a := SetDebugLevel{level} // Synchronous call
	client, err := rpc.DialHTTP(fam, server)
	if err != nil {
		log.Fatal("SetDebugLevelRPC: dialing: ", err)
	}
	err = client.Call("Node.Debug", a, &ans)
	if err != nil {
		log.Fatal("error:", err)
	}
	log.Printf("Was %d is %d\n", ans.level, level)
}

func getConfig() (config gpconfig) {
	for _, s := range []string{"gpconfig", "/etc/clustermatic/gpconfig"} {
		configdata, _ := ioutil.ReadFile(s)
		if configdata == nil {
			continue
		}
		err := json.Unmarshal(configdata, &config)
		if err != nil {
			log.Fatal("Bad config file:", err)
		}
		Dprintf(2, "config is %v\n", config)
		break
	}
	return
}
