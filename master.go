package main

import (
	"os"
	"log"
	"fmt"
	"io/ioutil"
)

var Workers []Worker

func startMaster(domainSock string) {
	log.SetPrefix("master " + *prefix + ": ")
	Dprintln(2, "starting master")

	go receiveCmds(domainSock)
	registerSlaves()
}

func receiveCmds(domainSock string) os.Error {
	l, err := Listen("unix", domainSock)
	if err != nil {
		log.Exit("listen error:", err)
	}
	for {
		var a StartReq
		c, err := l.Accept()
		if err != nil {
			log.Exitf("receiveCmds: accept on (%v) failed %v\n", l, err)
		}
		r := NewRpcClientServer(c)
		go func() {
			var Peer string = ""
			var newPeers []string
			var peerCount int
			r.Recv("receiveCmds", &a)
			// get credentials later
			/* Ye Olde State Machinee */
			for _, n := range a.Nodes {
				s, ok := slaves.Get(n)
				if !ok {
					log.Printf("No slave %v\n", n)
					continue
				}
				Dprintf(2, "node %v == slave %v\n", n, s)
				if Peer == "" {
					Peer = s.Server
					newPeers = []string(nil)
					peerCount++
				} else {
					newPeers = append(newPeers, s.Server)
					peerCount++
				}
				if peerCount >= *chainWorkers {
						na := a
						a.Nodes = nil
						a.Peers = newPeers
						cacheRelayFilesAndDelegateExec(&na, "", Peer)
						Peer = ""
						peerCount = 0
				}
			}

			if Peer != "" {
					na := a
					a.Nodes = nil
					a.Peers = newPeers
					cacheRelayFilesAndDelegateExec(&na, "", Peer)
			}
			r.Send("receiveCmds", Resp{Msg: []byte("cacheRelayFilesAndDelegateExec finished")})
		}()
	}
	return nil
}

func registerSlaves() os.Error {
	l, err := Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		log.Exit("listen error:", err)
	}
	Dprint(2, l.Addr())
	fmt.Println(l.Addr())
	err = ioutil.WriteFile("/tmp/srvaddr", []byte(l.Addr().String()), 0644)
	if err != nil {
		log.Exit(err)
	}
	slaves = newSlaves()
	for {
		c, err := l.Accept()
		if err != nil {
			log.Exit("registerSlaves:", err)
		}
		r := NewRpcClientServer(c)
		var req SlaveReq
		r.Recv("registerSlaves", &req)
		resp := slaves.Add(&req, r)
		r.Send("registerSlaves", resp)
	}
	return nil
}


type Slaves struct {
	slaves map[string]*SlaveInfo
}

func newSlaves() (s Slaves) {
	s.slaves = make(map[string]*SlaveInfo)
	return
}

func (sv *Slaves) Add(arg *SlaveReq, r *RpcClientServer) (resp SlaveResp) {
	var s *SlaveInfo
	if arg.id != "" {
		s = sv.slaves[arg.id]
	} else {
		s = &SlaveInfo{
			id:     fmt.Sprintf("%d", len(sv.slaves)+1),
			Addr:   arg.a,
			Server: arg.Server,
			rpc:    r,
		}
		sv.slaves[s.id] = s
	}
	Dprintln(2, "slave Add: id: ", s)
	resp.id = s.id
	return
}

func (sv *Slaves) Get(n string) (s *SlaveInfo, ok bool) {
	s, ok = sv.slaves[n]
	return
}

var slaves Slaves



func newStartReq(arg *StartReq) *StartReq {
	return &StartReq{
 		ThisNode:        true,
 		LocalBin:        arg.LocalBin,
		Peers:	arg.Peers,
 		Args:            arg.Args,
 		Env:             arg.Env,
 		Lfam:            arg.Lfam,
 		Lserver:         arg.Lserver,
 		cmds:            arg.cmds,
 		bytesToTransfer: arg.bytesToTransfer,
		chainWorkers: arg.chainWorkers,
 	}
}

func cacheRelayFilesAndDelegateExec(arg *StartReq, root, Server string) os.Error {
	Dprint(2, "cacheRelayFilesAndDelegateExec: files ", arg.cmds, " nodes: ", Server, " fileServer: ", arg.Lfam, arg.Lserver)

	larg := newStartReq(arg)
	client, err := Dial("tcp4", "", Server)
	if err != nil {
		log.Exit("dialing:", err)
	}
	Dprintf(2, "connected to %v\n", client)
	rpc := NewRpcClientServer(client)
	Dprintf(2, "rpc client %v, arg %v", rpc, larg)
	rpc.Send("cacheRelayFilesAndDelegateExec", larg)
	Dprintf(2, "bytesToTransfer %v localbin %v\n", arg.bytesToTransfer, arg.LocalBin)
	if arg.LocalBin {
		Dprintf(2, "cmds %v\n", arg.cmds)
	}
	writeOutFiles(rpc, root, arg.cmds)
	Dprintf(2, "cacheRelayFilesAndDelegateExec DONE\n")
	/* at this point it is out of our hands */

	return nil
}
