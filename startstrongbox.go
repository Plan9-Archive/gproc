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
	"fmt"
	"flag"
)

func usage() {
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	lowNode	=	flag.Int("l", 1, "Lowest node number")
	highNode=flag.Int("h", 196, "Highest node number")
)

func runlevel(lowNode, highNode int, mod7 bool) {
	for i := lowNode; i <= highNode; i++ {
		fmt.Printf("Check %d; mode %v; mod7 %v\n", i, (i %7 == 0), mod7)
		if (i % 7 == 0) != mod7  {
			continue
		}
		go func(anode int) {
			node := fmt.Sprintf("root@cn%d", anode)
			
			Args := []string{"ssh", "-o", "StrictHostKeyCHecking=no", node, "./gproc_linux_arm", "-locale=strongbox", "s"}
			f := []*os.File {nil, os.Stdout, os.Stderr}
			fmt.Printf("Spawn to %v\n", node)
			_, err := os.ForkExec("/usr/bin/ssh", Args, os.Environ(), "", f)
			if err != nil {
				fmt.Print("Forkexec fails: ", err)
			}
		}(i)
	}
}
func main() {
	flag.Usage = usage
	flag.Parse()
	fmt.Printf("Start nodes %d to %d\n", *lowNode, *highNode)
	runlevel(*lowNode, *highNode, true)
	fmt.Printf("Start nodes %d to %d\n", *lowNode, *highNode)
	runlevel(*lowNode, *highNode, false)
}

