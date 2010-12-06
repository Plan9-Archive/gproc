package main

import (
	"fmt"
	"log"
	"io"
	"exec"
)

var id string

func startSlave(rfam, raddr, srvaddr string) {
	newListenProc("slaveProc", slaveProc, srvaddr)
	client, err := Dial(rfam, "", raddr)
	if err != nil {
		log.Exit("dialing:", err)
	}
	r := NewRpcClientServer(client)
	req := &SlaveReq{}
	r.Send("startSlave", req)
	resp := &SlaveResp{}
	r.Recv("startSlave", &resp)
	id = resp.id
	log.SetPrefix("slave " + id + ": ")
	slaveProc(r)
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
	Dprintln(2, "ForkRelay: ", req.Nodes, "fileServer: ", req)
	argv := []string{"gproc",
		fmt.Sprintf("-debug=%d", *DebugLevel),
		fmt.Sprintf("-p=%v", *DoPrivateMount),
		"-prefix=" + id,
		"R",
	}
	nilEnv := []string{""}
	p, err := exec.Run("./gproc", argv, nilEnv, "", exec.Pipe, exec.PassThrough, exec.PassThrough)
	if err != nil {
		log.Exit("ForkRelay: run: ", err)
	}
	Dprintf(2, "forked %d\n", p.Pid)
	go WaitAllChildren()

	/* relay data to the child */
	if req.LocalBin {
		Dprintf(2, "ForkRelay arg.LocalBin %v arg.cmds %v\n", req.LocalBin, req.cmds)
	}
	rrpc := NewRpcClientServer(p.Stdin)
	rrpc.Send("ForkRelay", req)
	Dprintf(2, "clone pid %d err %v\n", p.Pid, err)
	n, err := io.Copy(rrpc.ReadWriter(), rpc.ReadWriter())
	Dprint(2, "ForkRelay: copy wrote ", n)
	if err != nil {
		log.Exit("ForkRelay: copy: ", err)
	}
	Dprint(2, "ForkRelay: end")
}
