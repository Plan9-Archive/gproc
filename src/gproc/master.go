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
)

var (
	Workers     []Worker
	netaddr     = ""
	exceptFiles map[string]bool
	exceptList  []string
)

func startMaster() {
	log.SetPrefix("master " + *prefix + ": ")
	log_info("starting master")
	exceptFiles = make(map[string]bool, 16)
	exceptList = []string{}

	go web()
	go receiveCmds(*defaultMasterUDS)
	registerSlaves()
}

func sendCommandsToANodeSet(sendReq *StartReq, subNodes string, root string, nodeSet []string) (numnodes int) {
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

	log_info("receiveCmds: slaveNodes: ", slaves, " nodeSet: ", nodeSet, " subnodes ", subNodes)

	sendReq.Nodes = subNodes
	for _, s := range nodeSet {
		if cacheRelayFilesAndDelegateExec(sendReq, root, s) == nil {
			numnodes += connsperNode
		} else {
			log_info(s, " failed")
			si, ok := slaves.Get(s)
			if ok {
				log_info("Remove slave ", s, " ", si)
				slaves.Remove(si)
			} else {
				log_info("Could not find slave ", s, " to remove")
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
	log_info("receiveCmds: sendReq.Nodes: ", sendReq.Nodes, " expands to ", slaveNodes)
	if err != nil {
		r.Send("receiveCmds", Resp{NumNodes: 0, Msg: "startExecution: bad slaveNodeList: " + err.Error()})
		return
	}
	for _, aNode := range slaveNodes {
		/* would be nice to spawn these async but we need the 
		 * nodecount ...
		 */
		nodeSet := slaves.ServIntersect(aNode.Nodes)
		numnodes += sendCommandsToANodeSet(sendReq, aNode.Subnodes, root, nodeSet)
	}
	log_info("numnodes = ", numnodes)
	return
}

/*
 * The master sits in a loop listening for commands to come in over the Unix domain socket.
 */
func receiveCmds(domainSock string) error {
	vitalData := vitalData{HostAddr: "", HostReady: false, Error: "No hosts ready", Exceptlist: exceptFiles}
	l, err := Listen("unix", *defaultMasterUDS)
	if err != nil {
		log_error("listen error:", err)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			log_error("receiveCmds: accept on (%v) failed %v\n", l, err)
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
					log_info("Respond to except request ", exceptOK)
					r.Send("exceptOK", exceptOK)
				}
			case a.Command[0] == uint8('i'):
				{
					hostinfo := Resp{}
					for i, s := range slaves.Addr2id {
						hostinfo.Msg += i + " " + s + "\n"
					}
					hostinfo.NumNodes = len(slaves.Addr2id)
					log_info("Respond to info request ", hostinfo)
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
