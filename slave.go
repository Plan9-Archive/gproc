package main

import (
	"fmt"
	"log"
	"io"
	"exec"
)

var id string

func startSlave(fam, masterAddr, peerAddr string) {
	serverAddr := newListenProc("slaveProc", slaveProc, peerAddr)
	client, err := Dial(fam, "", masterAddr)
	if err != nil {
		log.Exit("dialing:", err)
	}
	r := NewRpcClientServer(client)
	initSlave(r, serverAddr)
	slaveProc(r)
}

func initSlave(r *RpcClientServer, serverAddr string) {
	req := &SlaveReq{Server: serverAddr}
	r.Send("startSlave", req)
	resp := &SlaveResp{}
	r.Recv("startSlave", &resp)
	id = resp.id
	log.SetPrefix("slave " + id + ": ")
}

func slaveProc(r *RpcClientServer) {
	for {
		req := &StartReq{}
		// receives from cacheRelayFilesAndDelegateExec?
		r.Recv("slaveProc", req)
		ForkRelay(req, r)
		r.Send("slaveProc", Resp{Msg: []byte("slave finished")})
	}
}

func ForkRelay(req *StartReq, rpc *RpcClientServer) {
	Dprintln(2, "ForkRelay: ", req.Nodes, " fileServer: ", req)
	p := startRelay()
	/* relay data to the child */
	if req.LocalBin {
		Dprintf(2, "ForkRelay arg.LocalBin %v arg.cmds %v\n", req.LocalBin, req.cmds)
	}
	rrpc := NewRpcClientServer(p.Stdin)
	rrpc.Send("ForkRelay", req)
	// receives from cacheRelayFilesAndDelegateExec?
	n, err := io.Copyn(rrpc.ReadWriter(), rpc.ReadWriter(), req.bytesToTransfer)
	Dprint(2, "ForkRelay: copy wrote ", n)
	if err != nil {
		log.Exit("ForkRelay: copy: ", err)
	}
	Dprint(2, "ForkRelay: end")
}

func startRelay() *exec.Cmd {
	Dprintf(2, "startRelay:  starting")
	argv := []string{
		"gproc",
		fmt.Sprintf("-debug=%d", *DebugLevel),
		fmt.Sprintf("-p=%v", *DoPrivateMount),
		"-prefix=" + id,
		"R",
	}
	nilEnv := []string{""}
	p, err := exec.Run("./gproc", argv, nilEnv, "", exec.Pipe, exec.Pipe, exec.PassThrough)
	if err != nil {
		log.Exit("startRelay: run: ", err)
	}	
	Dprintf(2, "startRelay: forked %d\n", p.Pid)
	go WaitAllChildren()
	return p
}
