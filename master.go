package main

import (
	"os"
	"log"
	"fmt"
	"io/ioutil"
	"strings"
)

var (
	Workers []Worker
	netaddr = ""
)

func startMaster(domainSock string) {
	log.SetPrefix("master " + *prefix + ": ")
	Dprintln(2, "starting master")

	go receiveCmds(domainSock)
	registerSlaves()
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
			r.Recv("receiveCmds", &a)

			// get credentials later
			switch {
			case *peerGroupSize == 0:
				availableSlaves := slaves.ServIntersect(a.Nodes)
				Dprint(2, "receiveCmds: a.Nodes: ", a.Nodes, " availableSlaves: ", availableSlaves)

				a.Nodes = nil
				for _, s := range availableSlaves {
					na := a // copy argument
					cacheRelayFilesAndDelegateExec(&na, "", s)
				}
			default:
				availableSlaves := slaves.ServIntersect(a.Nodes)
				Dprint(2, "receiveCmds: peerGroup > 0 a.Nodes: ", a.Nodes, " availableSlaves: ", availableSlaves)

				a.Nodes = nil
				for len(availableSlaves) > 0 {
					numWorkers := *peerGroupSize
					if numWorkers > len(availableSlaves) {
						numWorkers = len(availableSlaves)
					}
					// the first available node is the server, the rest of the reservation are peers
					a.Peers = availableSlaves[1:numWorkers]
					na := a // copy argument
					cacheRelayFilesAndDelegateExec(&na, "", availableSlaves[0])
					availableSlaves = availableSlaves[numWorkers:]
				}
			}
			r.Send("receiveCmds", Resp{Msg: []byte("cacheRelayFilesAndDelegateExec finished")})
		}()
	}
	return nil
}

func registerSlaves() os.Error {
	l, err := Listen(defaultFam, cmdSocket)
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
		/* quite the hack. At some point, on a really complex system, 
		 * we'll need to return a set of listen addresses for a daemon, but we've yet to
		 * see that in actual practice. 
		 */
		if netaddr == "" {
			addr := strings.Split(c.LocalAddr().String(),":",2)
			netaddr = addr[0]
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
