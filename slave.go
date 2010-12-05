package main

import (
	"fmt"
	"log"
	"os"
	"io"
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
		ForkAndRelay(req, r)
		r.Send("slaveProc", Resp{Msg: []byte("slave finished")})
	}
}

func ForkAndRelay(req *StartReq, rpc *RpcClientServer) {
	Dprintln(2, "ForkAndRelay: ", req.Nodes, "fileServer: ", req)
	/* set up a pipe */
	r, w, err := os.Pipe()
	if err != nil {
		log.Exit("ForkAndRelay: ", err)
	}	
	bugger := fmt.Sprintf("-debug=%d", *DebugLevel)
	private := fmt.Sprintf("-p=%v", *DoPrivateMount)
	argv := []string{"gproc", bugger, private, "-prefix="+id, "R"}
	pid, err := os.ForkExec("./gproc", argv, []string{""}, "", []*os.File{r, w, w})
	defer r.Close()
	defer w.Close()
	if err != nil {
		log.Exit("ForkAndRelay: ", err)
	}
	Dprintf(2, "forked %d\n", pid)
	go Wait4()

	/* relay data to the child */
	if req.LocalBin {
		Dprintf(2, "ForkAndRelay arg.LocalBin %v arg.cmds %v\n", req.LocalBin, req.cmds)
	}
	rrpc := NewRpcClientServer(w)
	rrpc.Send("ForkAndRelay", req)
	Dprintf(2, "clone pid %d err %v\n", pid, err)
	n, err := io.Copy(rrpc.ReadWriter(), rpc.ReadWriter())
	Dprint(2, "ForkAndRelay: copy wrote ", n)
	if err != nil {
		log.Exit("ForkAndRelay: ", err)
	}
	Dprint(2, "ForkAndRelay: end")
}
