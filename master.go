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
	"log"
	"strings"
)

var (
	Workers []Worker
	netaddr = ""
)

func startMaster(domainSock string, loc Locale) {
	log.SetPrefix("master " + *prefix + ": ")
	Dprintln(2, "starting master")

	go receiveCmds(domainSock)
	registerSlaves(loc)
}

func sendCommands(r *RpcClientServer, sendReq *StartReq) {
			slaveNodes, err := parseNodeList(sendReq.Nodes)
			if err != nil {
				r.Send("receiveCmds", Resp{Msg: "startExecution: bad slaveNodeList: " + err.String()})
				return
			}
			Dprint(2, "receiveCmds: sendReq.Nodes: ", sendReq.Nodes, " expands to ", slaveNodes)
			// get credentials later
			switch {
			case *peerGroupSize == 0:
				availableSlaves := slaves.ServIntersect(slaveNodes[0].nodes)
				Dprint(2, "receiveCmds: slaveNodes: ", slaveNodes, " availableSlaves: ", availableSlaves, " subnodes " , slaveNodes[0].subnodes)

				sendReq.Nodes = slaveNodes[0].subnodes
				for _, s := range availableSlaves {
					cacheRelayFilesAndDelegateExec(sendReq, "", s)
				}
			default:
				availableSlaves := slaves.ServIntersect(slaveNodes[0].nodes)
				Dprint(2, "receiveCmds: peerGroup > 0 slaveNodes: ", slaveNodes, " availableSlaves: ", availableSlaves)

				sendReq.Nodes = slaveNodes[0].subnodes
				for len(availableSlaves) > 0 {
					numWorkers := *peerGroupSize
					if numWorkers > len(availableSlaves) {
						numWorkers = len(availableSlaves)
					}
					// the first available node is the server, the rest of the reservation are peers
					sendReq.Peers = availableSlaves[1:numWorkers]
					na := *sendReq // copy argument
					cacheRelayFilesAndDelegateExec(&na, "", availableSlaves[0])
					availableSlaves = availableSlaves[numWorkers:]
				}
			}
}

func receiveCmds(domainSock string) os.Error {
	vitalData := vitalData{HostAddr: "", HostReady: false, Error: "No hosts ready"}
	l, err := Listen("unix", domainSock)
	if err != nil {
		log.Exit("listen error:", err)
	}
	for {
		c, err := l.Accept()
		if err != nil {
			log.Exitf("receiveCmds: accept on (%v) failed %v\n", l, err)
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
			if ! vitalData.HostReady {
				return
			}
			/* it would be Really Cool if we could case out on the type of the request, I don't know how. */
			r.Recv("receiveCmds", &a)
			/* we could used re matching but that package is a bit big */
			switch {
				case a.Command[0] == uint8('i'): {
					hostinfo := Resp{}
					for i, s := range slaves.addr2id {
						hostinfo.Msg += i + " " + s + "\n"
					}
					r.Send("receiveCmds", hostinfo)
				}
				case a.Command[0] == uint8('e'): {
					sendCommands(r, &a)
					r.Send("receiveCmds", Resp{Msg: "cacheRelayFilesAndDelegateExec finished"})
				}
				default: {
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
		log.Exit("listen error:", err)
	}
	Dprint(2, l.Addr())
	err = loc.RegisterServer(l)
	if err != nil {
		log.Exit(err)
	}

	slaves = newSlaves()
	for {
		vd := &vitalData{}
		c, err := l.Accept()
		if err != nil {
			log.Exit("registerSlaves:", err)
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
	slaves map[string]*SlaveInfo
	addr2id map[string]string
}

func newSlaves() (s Slaves) {
	s.slaves = make(map[string]*SlaveInfo)
	return
}

func (sv *Slaves) Add(vd *vitalData, r *RpcClientServer) (resp SlaveResp) {
	var s *SlaveInfo
		s = &SlaveInfo{
			id:     loc.SlaveIdFromVitalData(vd),
			Addr:   vd.HostAddr, 
			Server: vd.ServerAddr,
			Nodes: vd.Nodes,
			rpc:    r,
		}
		sv.slaves[s.id] = s
	Dprintln(2, "slave Add: id: ", s)
	resp.id = s.id
	return
}

func (sv *Slaves) Get(n string) (s *SlaveInfo, ok bool) {
	s, ok = sv.slaves[n]
	return
}

func (sv *Slaves) ServIntersect(set []string) (i []string) {
	for _, n := range set {
		s, ok := sv.Get(n)
		if !ok {
			continue
		}
		i = append(i, s.Server)
	}
	return
}

var slaves Slaves


func newStartReq(arg *StartReq) *StartReq {
	return &StartReq{
		Command: arg.Command,
		Nodes: arg.Nodes,
		ThisNode:        true,
		LocalBin:        arg.LocalBin,
		Peers:           arg.Peers,
		Args:            arg.Args,
		Env:             arg.Env,
		LibList:	arg.LibList,
		Path:		arg.Path,
		Lfam:            arg.Lfam,
		Lserver:         arg.Lserver,
		cmds:            arg.cmds,
		bytesToTransfer: arg.bytesToTransfer,
		peerGroupSize:   arg.peerGroupSize,
	}
}
