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
	"log"
)

var (
	Workers     []Worker
	netaddr     = ""
	exceptFiles map[string]bool
	exceptList  []string
)

func startMaster(domainSock string, loc Locale) {
	log.SetPrefix("master " + *prefix + ": ")
	Dprintln(2, "starting master")
	exceptFiles = make(map[string]bool, 16)
	exceptList = []string{}

	go receiveCmds(domainSock)
	registerSlaves(loc)
}

func sendCommandsToANode(sendReq *StartReq, subNodes string, root string, availableSlaves []string) (numnodes int) {
	/* for efficiency, on the slave node, if there is one proc, 
	 * it connects directly to the parent IO forwarder. 
	 * If the slave node is tasking other nodes, it will also spawn
	 * off its own IO forwarder. The result is that there will be one
	 * or two connections from a slave node. There is no clear
	 * universal rule for what is the right thing to do, so 
	 * we just have to track how many connections to expect 
	 * from each slave node. That will be determined 
	 * by whether a slave node has peers or tasking to its own nodes. 
	 * This is kludgy, but again, it's not clear what the Best Choice is.
	 */
	connsperNode := 1

	Dprint(2, "receiveCmds: slaveNodes: ", slaves, " availableSlaves: ", availableSlaves, " subnodes ", subNodes)

	sendReq.Nodes = subNodes
	for _, s := range availableSlaves {
		if cacheRelayFilesAndDelegateExec(sendReq, root, s) == nil {
			numnodes += connsperNode
		} else {
			si, err := slaves.Get(s)
			if err {
				slaves.Remove(si)
			}
		}
	}

	return
}

/*
 * The master calls this to distribute commands and files to its sub-nodes
 */
func sendCommandsToNodes(r *RpcClientServer, sendReq *StartReq, root string) (numnodes int) {
	slaveNodes, err := parseNodeList(sendReq.Nodes)
	Dprint(2, "receiveCmds: sendReq.Nodes: ", sendReq.Nodes, " expands to ", slaveNodes)
	if err != nil {
		r.Send("receiveCmds", Resp{NumNodes: 0, Msg: "startExecution: bad slaveNodeList: " + err.String()})
		return
	}
	for _, aNode := range slaveNodes {
		/* would be nice to spawn these async but we need the 
		 * nodecount ...
		 */
		availableSlaves := slaves.ServIntersect(aNode.Nodes)
		numnodes += sendCommandsToANode(sendReq, aNode.Subnodes, root, availableSlaves)
	}
	Dprint(2, "numnodes = ", numnodes)
	return
}

/*
 * The master sits in a loop listening for commands to come in over the Unix domain socket.
 */
func receiveCmds(domainSock string) os.Error {
	vitalData := vitalData{HostAddr: "", HostReady: false, Error: "No hosts ready", Exceptlist: exceptFiles}
	l, err := Listen("unix", domainSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			log.Fatalf("receiveCmds: accept on (%v) failed %v\n", l, err)
		}
		r := NewRpcClientServer(c, *binRoot)
		go func() {
			var a StartReq

			if netaddr != "" {
				vitalData.HostReady = true
				vitalData.Error = ""
				vitalData.HostAddr = netaddr
			}
			r.Send("vitalData", vitalData)
			/* it would be Really Cool if we could case out on the type of the request, I don't know how. */
			err := r.Recv("receiveCmds", &a)
			if err != nil {
				return
			}
			/* we could used re matching but that package is a bit big */
			switch {
			case a.Command[0] == uint8('x'):
				{
					for _, s := range a.Args {
						exceptFiles[s] = true
					}
					exceptList = []string{}
					for s, _ := range exceptFiles {
						exceptList = append(exceptList, s)
					}
					exceptOK := Resp{Msg: "Files accepted"}
					Dprint(8, "Respond to except request ", exceptOK)
					r.Send("exceptOK", exceptOK)
				}
			case a.Command[0] == uint8('i'):
				{
					hostinfo := Resp{}
					for i, s := range slaves.Addr2id {
						hostinfo.Msg += i + " " + s + "\n"
					}
					hostinfo.NumNodes = len(slaves.Addr2id)
					Dprint(8, "Respond to info request ", hostinfo)
					r.Send("hostinfo", hostinfo)
				}
			case a.Command[0] == uint8('e'):
				{
					if !vitalData.HostReady {
						return
					}
					numnodes := sendCommandsToNodes(r, &a, "")
					r.Send("receiveCmds", Resp{NumNodes: numnodes, Msg: "sendCommandsToNodes finished"})
				}
			default:
				{
					r.Send("unknown command", Resp{Msg: "unknown command"})
				}
			}
		}()
	}
	return nil
}
