package main

import (
	"fmt"
	"log"
	"os"
	"io"
)

var id string
func slaveProc(r *RpcClientServer) {
	for {
		req := &StartReq{}
		// receives from MExec?
		r.Recv("slaveProc", req)
		RExec(req, r)
		r.Send("slaveProc", Resp{Msg: []byte("slave finished")})
	}
}

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


/* rexec will create a listener and then relay the results. We do this go get an IO hierarchy. */
func RExec(req *StartReq, rpc *RpcClientServer) (err os.Error) {
	Dprintln(2, "RExec: ", req.Nodes, "fileServer: ", req)
	/* set up a pipe */
	r, w, err := os.Pipe()
	if err != nil {
		log.Exit("RExec: ", err)
	}	
	bugger := fmt.Sprintf("-debug=%d", *DebugLevel)
	private := fmt.Sprintf("-p=%v", *DoPrivateMount)
	pid, err := os.ForkExec("./gproc", []string{"gproc", bugger, private, "-prefix="+id, "R"}, []string{""}, ".", []*os.File{r, os.Stdout, os.Stdout})
	r.Close()
	defer w.Close()
	if err != nil {
		log.Exit("RExec: ", err)
	}
	Dprintf(2, "forked %d\n", pid)
	go Wait4()

	/* relay data to the child */
	if req.LocalBin {
		Dprintf(2, "RExec arg.LocalBin %v arg.cmds %v\n", req.LocalBin, req.cmds)
	}
	rrpc := NewRpcClientServer(w)
	rrpc.Send("RExec", req)
	Dprintf(2, "clone pid %d err %v\n", pid, err)
	n, err := io.Copy(rrpc.ReadWriter(), rpc.ReadWriter())
	Dprint(2, "RExec copy wrote", n)
	if err != nil {
		log.Exit("RExec: ", err)
	}
	Dprint(2, "RExec: end")
	return
}
