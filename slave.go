package main

import (
	"fmt"
	"log"
	"os"
	"io"
)

var id string
func slaveserv(r *RpcClientServer) {
	for {
		arg := &StartArg{}
		r.Recv("slaveserv", arg)
		RExec(arg, r)
		r.Send("slaveserv", Res{Msg: []byte("slave finished")})
	}
}

func slave(rfam, raddr, srvaddr string) {
	newListenProc("slaveserv", slaveserv, srvaddr)
	client, err := Dial(rfam, "", raddr)
	if err != nil {
		log.Exit("dialing:", err)
	}
	r := NewRpcClientServer(client)
	a := &SlaveArg{}
	r.Send("slave", a)
	ans := &SlaveRes{}
	r.Recv("slave", &ans)
	id = ans.id
	log.SetPrefix("slave " + id + ": ")
	slaveserv(r)
}


/* rexec will create a listener and then relay the results. We do this go get an IO hierarchy. */
func RExec(arg *StartArg, rpc *RpcClientServer) (err os.Error) {
	Dprintln(2, "RExec: ", arg.Nodes, "fileServer: ", arg)
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
	if arg.LocalBin {
		Dprintf(2, "RExec arg.LocalBin %v arg.cmds %v\n", arg.LocalBin, arg.cmds)
	}
//	rpc.Send("RExec", arg)
	rrpc := NewRpcClientServer(w)
	rrpc.Send("RExec", arg)
	Dprintf(2, "clone pid %d err %v\n", pid, err)
	n, err := io.Copy(rrpc.ReadWriter(), rpc.ReadWriter())
	Dprint(2, "RExec copy wrote", n)
	if err != nil {
		log.Exit("RExec: ", err)
	}
	Dprint(2, "RExec: end")
	return
}
