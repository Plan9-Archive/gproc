/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the GNU Lesser General Public License, version 2, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */
/* 
 * parameters: 
 * expression to figure your level -- pass to forth
 * parent. expression to name your parent. 
 * id. expression to compute the id
 * myhost. expression to compute host. Default to 'hostname' since the host can work that out. 
 */
package main

import (
	"flag"
	"fmt"
	"os"
)

func usage() {
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	lowNode      = flag.Int("l", 1, "Lowest node number")
	highNode     = flag.Int("h", 40, "Highest node number")
	debugLevel   = flag.Int("d", 0, "Debug level")
	privateMount = flag.Bool("p", true, "private mounts")
	runornot     = flag.String("b", "20", "block size -- default 20")
	myAddress    = flag.String("myAddress", "hostname", "what to tell a node to compute it's own address")
)

func runlevel(lowNode, highNode int, mod bool) {
	reap := make(chan *os.Waitmsg, 0)
	numspawn := 0
	for i := lowNode; i <= highNode; i++ {
		fmt.Printf("Check %d; mode %v; mod %v\n", i, (i%BLOCKSIZE == 0), mod)
		if (i%BLOCKSIZE == 0) != mod {
			continue
		}
		numspawn++
		go func(anode int) {
			node := fmt.Sprintf("root@kn%d", anode)

			Args := []string{"ssh", "-o", "StrictHostKeyCHecking=no", node, "./gproc_linux_amd64",
				"-myParent='hostname base 20 roundup kn strcat 10.1.234.234 hostname base 20 % ifelse'",
				"-myId='hostname base 20 % 1  + hostname base 20 / hostname base    %  ifelse'",
				"-myAddress=hostname",
				fmt.Sprintf("-p=%v ", *privateMount), fmt.Sprintf("-debug=%d", *debugLevel), "s"}
			f := []*os.File{nil, os.Stdout, os.Stderr}
			fmt.Printf("Spawn to %v\n", node)
			pid, err := os.StartProcess("/usr/bin/ssh", Args, &os.ProcAttr{Files: f})
			if err != nil {
				fmt.Print("StartProcess fails: ", err)
			}

			msg, err := os.Wait(pid.Pid, 0)
			reap <- msg
		}(i)
	}
	for numspawn > 0 {
		msg := <-reap
		fmt.Printf("Reaped %v\n", msg)
		numspawn--
	}
}
func main() {
	flag.Usage = usage
	flag.Parse()
	/* use 1 for the top level. Use anything else for the next level down */
	level1 := flag.Args()[0] == "1"
	fmt.Printf("Start nodes %d to %d\n", *lowNode, *highNode)
	runlevel(*lowNode, *highNode, level1)
}
