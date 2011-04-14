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
	"os"
	"fmt"
	"flag"
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
	locale       = flag.String("locale", "kf", "Locale")
	parent       = flag.String("parent", "10.1.254.254", "Parent")
	cmdPort      = flag.String("cmdport", "6666", "Command port")
)

func runlevel(lowNode, highNode int) {
	reap := make(chan *os.Waitmsg, 0)
	numspawn := 0
	for i := lowNode; i <= highNode; i++ {
		numspawn++
		go func(anode int) {
			node := fmt.Sprintf("root@kn%d", anode)

			Args := []string{"ssh", "-o", "StrictHostKeyCHecking=no", node, "./gproc_linux_amd64", fmt.Sprintf("-p=%v ", *privateMount), fmt.Sprintf("--cmdport=%s", *cmdPort), fmt.Sprintf("-locale=%s ", *locale), fmt.Sprintf("-parent=%s ", *parent), fmt.Sprintf("-debug=%d ", *debugLevel), "s"}
			fmt.Println(Args)
			f := []*os.File{nil, os.Stdout, os.Stderr}
			fmt.Printf("Spawn to %v\n", node)
			pid, err := os.StartProcess("/usr/bin/ssh", Args, &os.ProcAttr{Files: f})
			if err != nil {
				fmt.Print("Forkexec fails: ", err)
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
	fmt.Printf("Start nodes %d to %d\n", *lowNode, *highNode)
	runlevel(*lowNode, *highNode)
}
