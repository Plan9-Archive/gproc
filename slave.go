package main

import (
	"fmt"
	"log"
	"os"
	"net"
	"io"
)

func slaveserv(client net.Conn) {
	for {
		arg := &StartArg{}
		Recv("slave", client, arg)
		RExec(arg, client)
		res := &Res{Msg: []byte("slave finished")}
		Send("slave", client, res)
	}
}

func slave(rfam, raddr, srvaddr string) {
	newListenProc("slaveserv", slaveserv, srvaddr)
	client, err := net.Dial(rfam, "", raddr)
	if err != nil {
		log.Exit("dialing:", err)
	}
	a := &SlaveArg{}
	Send("slave", client, a)
	ans := &SlaveRes{}
	Recv("slave", client, &ans)
	log.SetPrefix("slave " + ans.id + ": ")
	slaveserv(client)
}


/* rexec will create a listener and then relay the results. We do this go get an IO hierarchy. */
func RExec(arg *StartArg, c net.Conn) (err os.Error) {
	Dprintln(2, "RExec: ", arg.Nodes, "fileServer: ", arg)
	/* set up a pipe */
	r, w, err := os.Pipe()
	if err != nil {
		log.Exit("RExec: ", err)
	}	
	bugger := fmt.Sprintf("-debug=%d", *DebugLevel)
	private := fmt.Sprintf("-p=%v", *DoPrivateMount)
	pid, err := os.ForkExec("./gproc", []string{"gproc", bugger, private, "R"}, []string{""}, ".", []*os.File{r, os.Stdout, os.Stdout})
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
	Send("RExec", w, arg)
	Dprintf(2, "clone pid %d err %v\n", pid, err)
	_, err = io.Copy(w, c)
	if err != nil {
		log.Exit("RExec: ", err)
	}
	Dprint(2, "RExec: end")
	return
}
