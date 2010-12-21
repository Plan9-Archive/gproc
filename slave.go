package main

import (
	"fmt"
	"log"
	"io"
	"exec"
	"strings"
	"os"
)

var id string

/* We will for now assume that addressing is symmetric, that is, if we Dial someone on
 * a certain address, that's the address they should Dial us on. This assumption has held
 * up well for quite some time. And, in fact, it makes no sense to do it any other way ...
 */
/* note that we're going to be able to merge master and slave fairly soon, now that they do almost the same things. */
func startSlave(fam, masterAddr string, loc Locale) {
	/* slight difference from master: we're ready when we start, since we run things */
	vitalData := &vitalData{HostReady: true}
	/* some simple sanity checking */
	if *DoPrivateMount == true && os.Getuid() != 0 {
		log.Exit("Need to run as root for private mounts")
	}
	client, err := Dial(fam, "", masterAddr)
	if err != nil {
		log.Exit("dialing:", err)
	}

	/* vitalData -- what we're doing here is assembling information for our parent. 
	 * we have to tell our parent what port we look for process startup commands on, 
	 * the address of our side of the Dial connection, and, due to a limitation in the Unix
	 * kernels going back a long time, we might as well tell the master its own address for
	 * the socket, since *the master can't get it*. True! 
	 */
	addr := strings.Split(client.LocalAddr().String(), ":", -1)
	peerAddr := addr[0] + ":0"
	vitalData.ServerAddr = newListenProc("slaveProc", slaveProc, peerAddr)
	vitalData.HostAddr = client.LocalAddr().String()
	vitalData.ParentAddr = client.RemoteAddr().String()
	r := NewRpcClientServer(client)
	initSlave(r, vitalData)
	go registerSlaves(loc)
	for {
		slaveProc(r)
	}
}

func initSlave(r *RpcClientServer, v *vitalData) {
	Dprint(2,"initSlave: ", v)
	r.Send("startSlave", *v)
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
	/* create the array of strings to send. You can't just send the slaveinfo struct as Go won't like that. 
	 * You don't have fork
	 * and you can't do it here as the child will build a private name space. 
	 * So take the req.Nodes, bust them into bits just as the master does, and create an array of 
	 * socket names {'"a.b.c.d/x"...} and the subnode names {"1-5"} and pass them down. 
	 * this is almost ready but it won't make it.
	 */
	ne, _ := parseNodeList(req.Nodes)
	nsend := nodeExecList{subnodes: ne[0].subnodes}
	nsend.nodes =  slaves.ServIntersect(ne[0].nodes)
	Dprint(2, "Parsed node list to ", ne, " and nsend is ", nsend)
	/* the run code will then have a list of servers and node list to send to them */
	rrpc.Send("ForkRelay", &nsend)
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
	// Argv[0] will not always be ./gproc ...
	p, err := exec.Run(os.Args[0], argv, nilEnv, "", exec.Pipe, exec.Pipe, exec.PassThrough)
	if err != nil {
		log.Exit("startRelay: run: ", err)
	}
	Dprintf(2, "startRelay: forked %d\n", p.Pid)
	go WaitAllChildren()
	return p
}
