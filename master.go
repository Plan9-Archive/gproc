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

/* the most complex one. Needs to ForkExec itself, after
 * pasting the fd for the accept over the stdin etc.
 * and the complication of course is that net.Conn is
 * not able to do this, we have to relay the data
 * via a pipe. Oh well, at least we get to manage the
 * net.Conn without worrying about child fooling with it. BLEAH.
 */
func startMaster(addr string) {
	log.SetPrefix("master " + *prefix + ": ")
	Dprintln(2, "starting master")
	l, err := Listen("unix", addr)
	if err != nil {
		log.Exit("listen error:", err)
	}

	go unixServe(l)

	netl, err := Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		log.Exit("listen error:", err)
	}
	Dprint(2, netl.Addr())
	fmt.Println(netl.Addr())
	err = ioutil.WriteFile("/tmp/srvaddr", []byte(netl.Addr().String()), 0644)
	if err != nil {
		log.Exit(err)
	}

	masterServe(netl)

}

func unixServe(l Listener) os.Error {
	for {
		var a StartReq
		c, err := l.Accept()
		if err != nil {
			log.Exitf("unixServe: accept on (%v) failed %v\n", l, err)
		}
		r := NewRpcClientServer(c)
		go func() {
			r.Recv("unixServe", &a)
			// get credentials later
			cacheRelayFilesAndDelegateExec(&a, r)
			r.Send("unixServe", Resp{Msg: []byte("cacheRelayFilesAndDelegateExec finished")})
		}()
	}
	return nil
}

/* you need to keep making new encode/decoders because the process
 * at the other end is always new
 */
func masterServe(l Listener) os.Error {
	for {
		c, err := l.Accept()
		if err != nil {
			log.Exit("masterServe:", err)
		}
		r := NewRpcClientServer(c)
		var a SlaveReq
		r.Recv("masterServe", &a)
		r.Send("masterServe", newSlave(&a, r))
	}
	return nil
}


func newStartReq(arg *StartReq) *StartReq {
	return &StartReq{
		ThisNode:       true,
		LocalBin:       arg.LocalBin,
		Args:           arg.Args,
		Env:            arg.Env,
		Lfam:           arg.Lfam,
		Lserver:        arg.Lserver,
		cmds:           arg.cmds,
		totalfilebytes: arg.totalfilebytes,
	}
}

func cacheRelayFilesAndDelegateExec(arg *StartReq, r *RpcClientServer) os.Error {
	Dprint(2, "cacheRelayFilesAndDelegateExec: ", arg.Nodes, " fileServer: ", arg.Lfam, arg.Lserver)

	// buffer files on master
	data := bytes.NewBuffer(make([]byte,0))
	Dprint(2, "cacheRelayFilesAndDelegateExec: doing copyn")
	n, err := io.Copyn(data, r.ReadWriter(), arg.totalfilebytes)
	Dprint(2, "cacheRelayFilesAndDelegateExec readbytes ", data.Bytes()[0:64])
	Dprint(2, "cacheRelayFilesAndDelegateExec: copied ", n, " total ", arg.totalfilebytes)
	if err != nil {
		log.Exit("Mexec: copyn: ", err)
	}

	/* this is explicitly for sending to remote nodes. So we actually just pick off one node at a time
	 * and call execclient with it. Later we will group nodes.
	 */
	Dprint(2, "cacheRelayFilesAndDelegateExec nodes ", arg.Nodes)
	for _, n := range arg.Nodes {
		s, ok := Slaves[n]
		Dprintf(2, "node %v == slave %v\n", n, s)
		if !ok {
			log.Printf("No slave %v\n", n)
			continue
		}
		larg := newStartReq(arg)
		s.rpc.Send("cacheRelayFilesAndDelegateExec", larg)
		Dprintf(2, "totalfilebytes %v localbin %v\n", arg.totalfilebytes, arg.LocalBin)
		if arg.LocalBin {
			Dprintf(2, "cmds %v\n", arg.cmds)
		}
		Dprint(2, "cacheRelayFilesAndDelegateExec bytes ", data.Bytes()[0:64])
		n, err := io.Copyn(s.rpc.ReadWriter(), bytes.NewBuffer(data.Bytes()), arg.totalfilebytes)
		Dprint(2, "cacheRelayFilesAndDelegateExec: wrote ", n)
		if err != nil {
			log.Exit("cacheRelayFilesAndDelegateExec: iocopy: ", err)
		}
		/* at this point it is out of our hands */
	}

	return nil
}


func newSlave(arg *SlaveReq, r *RpcClientServer) (resp SlaveResp) {
	var s *SlaveInfo
	if arg.id != "" {
		s = Slaves[arg.id]
	} else {
		s = &SlaveInfo{
			id:     fmt.Sprintf("%d", len(Slaves)+1),
			Addr:   arg.a,
			Server: arg.Server,
			rpc:    r,
		}
		Slaves[s.id] = s
	}

	Dprintln(2, "startSlave: id: ", s)
	resp.id = s.id
	return
}

