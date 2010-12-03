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

type noderange struct {
	Base int
	Ip   string
}

type gpconfig struct {
	Noderanges []noderange
}

func usage() {
	fmt.Fprint(os.Stderr, "usage: gproc m <path>\n")
	fmt.Fprint(os.Stderr, "usage: gproc s <family> <address> <server address>\n")
	fmt.Fprint(os.Stderr, "usage: gproc e <server address> <fam> <address> <nodes> <command>\n")
	fmt.Fprint(os.Stderr, "usage: gproc R\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	Logfile = "/tmp/log"

	localbin       = flag.Bool("localbin", false, "execute local files")
	DoPrivateMount = flag.Bool("p", false, "Do a private mount")
	DebugLevel     = flag.Int("debug", 0, "debug level")
	/* this one gets me a zero-length string if not set. Phooey. */
	takeout = flag.String("f", "", "comma-seperated list of files/directories to take along")
	root    = flag.String("r", "", "root for finding binaries")
	libs    = flag.String("L", "/lib:/usr/lib", "library path")
)

func main() {
	flag.Usage = usage
	flag.Parse()
	log.SetPrefix("newgproc: ")
	Slaves = make(map[string]SlaveInfo, 1024)
	//setupLog()
	//config := getConfig()
	Dprintln(2, "starting:", os.Args,"debuglevel", *DebugLevel)

	switch flag.Arg(0) {
	/* traditional bproc master, commands over unix domain socket */
	case "DEBUG", "debug", "d":
		SetDebugLevelRPC(flag.Arg(1), flag.Arg(2), flag.Arg(3))
	case "MASTER", "master", "m":
		if len(flag.Args()) < 2 {
			flag.Usage()
		}
		master(flag.Arg(1))
	case "WORKER", "worker", "s":
		/* traditional slave; connect to master, await instructions */
		if len(flag.Args()) < 4 {
			flag.Usage()
		}
		slave(flag.Arg(1), flag.Arg(2), flag.Arg(3))
	case "EXEC", "exec", "e":
		if len(flag.Args()) < 6 {
			flag.Usage()
		}
		mexec(flag.Arg(1), flag.Arg(2), flag.Arg(3), flag.Arg(4), flag.Args()[5:])
	case "RUN", "run", "R":
		run()
	default:
		flag.Usage()
	}
}

func setupLog() {
	logfile, err := os.Open(Logfile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Exit("No log file", err)
	}

	log.SetOutput(logfile)

	log.Printf("DoPrivateMount: %v\n", DoPrivateMount)
}

func SetDebugLevelRPC(fam, server, newlevel string) {
	var ans SetDebugLevel
	level, err := strconv.Atoi(newlevel)
	if err != nil {
		log.Exit("bad level:", err)
	}

	a := SetDebugLevel{level} // Synchronous call
	client, err := rpc.DialHTTP(fam, server)
	if err != nil {
		log.Exit("dialing:", err)
	}
	err = client.Call("Node.Debug", a, &ans)
	if err != nil {
		log.Exit("error:", err)
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
			log.Exit("Bad config file:", err)
		}
		Dprintf(2,"config is %v\n", config)
		break
	}
	return
}
