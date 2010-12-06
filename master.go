package main

import (
	"os"
	"log"
	"fmt"
	"io/ioutil"
	"io"
	"bytes"
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
			r.Recv("receiveCmds", &a)
			// get credentials later
			cacheRelayFilesAndDelegateExec(&a, r)
			r.Send("receiveCmds", Resp{Msg: []byte("cacheRelayFilesAndDelegateExec finished")})
		}()
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

func newStartReq(arg *StartReq) *StartReq {
	return &StartReq{
		ThisNode:        true,
		LocalBin:        arg.LocalBin,
		Args:            arg.Args,
		Env:             arg.Env,
		Lfam:            arg.Lfam,
		Lserver:         arg.Lserver,
		cmds:            arg.cmds,
		bytesToTransfer: arg.bytesToTransfer,
	}
}

func cacheRelayFilesAndDelegateExec(arg *StartReq, r *RpcClientServer) os.Error {
	Dprint(2, "cacheRelayFilesAndDelegateExec: ", arg.Nodes, " fileServer: ", arg.Lfam, arg.Lserver)

	// buffer files on master
	data := bytes.NewBuffer(make([]byte, 0))
	Dprint(2, "cacheRelayFilesAndDelegateExec: copying ", arg.bytesToTransfer)
	n, err := io.Copyn(data, r.ReadWriter(), arg.bytesToTransfer)
	//		n, err := io.Copy(data, r.ReadWriter())
	Dprint(2, "cacheRelayFilesAndDelegateExec readbytes ", data.Bytes()[0:64])
	Dprint(2, "cacheRelayFilesAndDelegateExec: copied ", n, " total ", arg.bytesToTransfer)
	if err != nil {
		log.Exit("Mexec: copyn: ", err)
	}

	/* this is explicitly for sending to remote nodes. So we actually just pick off one node at a time
	 * and call execclient with it. Later we will group nodes.
	 */
	Dprint(2, "cacheRelayFilesAndDelegateExec nodes ", arg.Nodes)
	for _, n := range arg.Nodes {
		s, ok := slaves.Get(n)
		Dprintf(2, "node %v == slave %v\n", n, s)
		if !ok {
			log.Printf("No slave %v\n", n)
			continue
		}
		larg := newStartReq(arg)
		s.rpc.Send("cacheRelayFilesAndDelegateExec", larg)
		Dprintf(2, "bytesToTransfer %v localbin %v\n", arg.bytesToTransfer, arg.LocalBin)
		if arg.LocalBin {
			Dprintf(2, "cmds %v\n", arg.cmds)
		}
		Dprint(2, "cacheRelayFilesAndDelegateExec bytes ", data.Bytes()[0:64])
		n, err := io.Copyn(s.rpc.ReadWriter(), bytes.NewBuffer(data.Bytes()), arg.bytesToTransfer)
		//			n, err := io.Copy(s.rpc.ReadWriter(), bytes.NewBuffer(data.Bytes()))
		Dprint(2, "cacheRelayFilesAndDelegateExec: wrote ", n)
		if err != nil {
			log.Exit("cacheRelayFilesAndDelegateExec: iocopy: ", err)
		}
		/* at this point it is out of our hands */
	}

	return nil
}
