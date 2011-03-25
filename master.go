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
	"strings"
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

func sendCommandsToANode(sendReq *StartReq, aNode nodeExecList) (numnodes int) {
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
	if len(aNode.Subnodes) > 0 {
		connsperNode = 2
	}
	// get credentials later
	switch {
	case sendReq.PeerGroupSize == 0:
		availableSlaves := slaves.ServIntersect(aNode.Nodes)
		Dprint(2, "receiveCmds: slaveNodes: ", aNode.Nodes, " availableSlaves: ", availableSlaves, " subnodes ", aNode.Subnodes)
		
		sendReq.Nodes = aNode.Subnodes
		for _, s := range availableSlaves {
			if cacheRelayFilesAndDelegateExec(sendReq, "", s) == nil {
				numnodes += connsperNode
			}
		}
	default:
		availableSlaves := slaves.ServIntersect(aNode.Nodes)
		Dprint(2, "receiveCmds: peerGroup > 0 slaveNodes: ", aNode.Nodes, " availableSlaves: ", availableSlaves)

		sendReq.Nodes = aNode.Subnodes
		for len(availableSlaves) > 0 {
			numWorkers := sendReq.PeerGroupSize
			if numWorkers > len(availableSlaves) {
				numWorkers = len(availableSlaves)
			}
			// the first available node is the server, the rest of the reservation are peers
			sendReq.Peers = availableSlaves[1:numWorkers]
			numnodes++
			if len(sendReq.Peers) > 0 {
				numnodes++
			}
			na := *sendReq // copy argument
			cacheRelayFilesAndDelegateExec(&na, "", availableSlaves[0])
			availableSlaves = availableSlaves[numWorkers:]
		}
	}
	return
}

func sendCommandsToNodes(r *RpcClientServer, sendReq *StartReq) (numnodes int) {
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
		numnodes += sendCommandsToANode(sendReq, aNode)
	}
	return
}

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
		r := NewRpcClientServer(c)
		go func() {
			var a StartReq

			if netaddr != "" {
				vitalData.HostReady = true
				vitalData.Error = ""
				vitalData.HostAddr = netaddr
			}
			r.Send("vitalData", vitalData)
			/* it would be Really Cool if we could case out on the type of the request, I don't know how. */
			r.Recv("receiveCmds", &a)
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
					for i, s := range slaves.addr2id {
						hostinfo.Msg += i + " " + s + "\n"
					}
					hostinfo.NumNodes = len(slaves.addr2id)
					Dprint(8, "Respond to info request ", hostinfo)
					r.Send("hostinfo", hostinfo)
				}
			case a.Command[0] == uint8('e'):
				{
					if !vitalData.HostReady {
						return
					}
					numnodes := sendCommandsToNodes(r, &a)
					r.Send("receiveCmds", Resp{NumNodes: numnodes, Msg: "cacheRelayFilesAndDelegateExec finished"})
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

/* move this to common once Noah has merged. */
func registerSlaves(loc Locale) os.Error {
	l, err := Listen(defaultFam, loc.Addr())
	if err != nil {
		log.Fatal("listen error:", err)
	}
	Dprint(0, "-cmdport=", l.Addr())
	Dprint(2, l.Addr())
	err = loc.RegisterServer(l)
	if err != nil {
		log.Fatal(err)
	}

	slaves = newSlaves()
	for {
		vd := &vitalData{}
		c, err := l.Accept()
		if err != nil {
			log.Fatal("registerSlaves:", err)
		}
		r := NewRpcClientServer(c)
		r.Recv("registerSlaves", &vd)
		/* quite the hack. At some point, on a really complex system, 
		 * we'll need to return a set of listen addresses for a daemon, but we've yet to
		 * see that in actual practice. We can't use LocalAddr here, since it returns our listen
		 * address, not the address we accepted on, and if that's 0.0.0.0, that's useless. 
		 */
		if netaddr == "" {
			addr := strings.Split(vd.ParentAddr, ":", 2)
			Dprint(2, "addr is ", addr)
			netaddr = addr[0]
		}
		/* depending on the machine we are on, it is possible we don't get a usable IP address 
		 * in the ServerAddr. We'll have a good port, however, In this case, we need
		 * to cons one up, which is easily done. 
		 */
		if vd.ServerAddr[0:len("0.0.0.0")] == "0.0.0.0" {
			vd.ServerAddr = strings.Split(c.RemoteAddr().String(), ":", 2)[0] + vd.ServerAddr[7:]
			Dprint(2, "Guessed remote slave ServerAddr is ", vd.ServerAddr)
		}
		resp := slaves.Add(vd, r)
		r.Send("registerSlaves", resp)
	}
	return nil
}


type Slaves struct {
	slaves  map[string]*SlaveInfo
	addr2id map[string]string
}

func newSlaves() (s Slaves) {
	s.slaves = make(map[string]*SlaveInfo)
	s.addr2id = make(map[string]string)
	return
}

func (sv *Slaves) Add(vd *vitalData, r *RpcClientServer) (resp SlaveResp) {
	var s *SlaveInfo
	s = &SlaveInfo{
		id:     loc.SlaveIdFromVitalData(vd),
		Addr:   vd.HostAddr,
		Server: vd.ServerAddr,
		Nodes:  vd.Nodes,
		rpc:    r,
	}
	sv.slaves[s.id] = s
	sv.addr2id[s.Server] = s.id
	Dprintln(2, "slave Add: id: ", s)
	resp.Id = s.id
	return
}

func (sv *Slaves) Get(n string) (s *SlaveInfo, ok bool) {
	s, ok = sv.slaves[n]
	return
}

/* a hack for now. Sorry, we need to clean up the whole parsenodelist/intersect thing
 * but I need something that works and we're still putting the ideas 
 * together. So sue me. 
 */
func (sv *Slaves) ServIntersect(set []string) (i []string) {
	switch set[0] {
	case ".":
		for _, n := range sv.slaves {
			i = append(i, n.Server)
		}
	default:
		for _, n := range set {
			s, ok := sv.Get(n)
			if !ok {
				continue
			}
			i = append(i, s.Server)
		}
	}
	return
}

var slaves Slaves


func newStartReq(arg *StartReq) *StartReq {
	return &StartReq{
		Command:         arg.Command,
		Nodes:           arg.Nodes,
		ThisNode:        true,
		LocalBin:        arg.LocalBin,
		Peers:           arg.Peers,
		Args:            arg.Args,
		Env:             arg.Env,
		LibList:         arg.LibList,
		Path:            arg.Path,
		Lfam:            arg.Lfam,
		Lserver:         arg.Lserver,
		Cmds:            arg.Cmds,
		BytesToTransfer: arg.BytesToTransfer,
		PeerGroupSize:   arg.PeerGroupSize,
		Cwd: arg.Cwd,
	}
}
