package main

import (
	"fmt"
	"log"
	"io"
	"exec"
	"strings"
)

var id string

/* We will for now assume that addressing is symmetric, that is, if we Dial someone on
 * a certain address, that's the address they should Dial us on. This assumption has held
 * up well for quite some time. And, in fact, it makes no sense to do it any other way ...
 */
func startSlave(fam, masterAddr string) {
	client, err := Dial(fam, "", masterAddr)
	if err != nil {
		log.Exit("dialing:", err)
	}
	addr := strings.Split(client.LocalAddr().String(), ":", -1)
	peerAddr := addr[0] + ":" + cmdPort
	serverAddr := newListenProc("slaveProc", slaveProc, peerAddr)
	r := NewRpcClientServer(client)
	initSlave(r, serverAddr)
	for {
		slaveProc(r)
	}
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
	req := &StartReq{}
	// receives from cacheRelayFilesAndDelegateExec?
	r.Recv("slaveProc", req)
	ForkRelay(req, r)
	/* well, *maybe* we should do this, but we're commenting it out for now ...
	 * none of the clients look for this message and in the case of a delegation
	 * we're getting EPIPE
	 *
	r.Send("slaveProc", Resp{Msg: []byte("slave finished")})
	*/
	Dprintln(2, "slaveProc: ", req, " ends\n")

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
		log.Exit("ForkRelay: io.Copyn write error: ", err)
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
