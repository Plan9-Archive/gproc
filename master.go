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
		var a StartArg
		c, err := l.Accept()
		if err != nil {
			log.Exitf("unixServe: accept on (%v) failed %v\n", l, err)
		}
		r := NewRpcClientServer(c)
		go func() {
			r.Recv("unixServe", &a)
			// get credentials later
			MExec(&a, r)
			r.Send("unixServe", Res{Msg: []byte("MExec finished")})
		}()
	}
	return nil
}

func MExec(arg *StartArg, r *RpcClientServer) os.Error {
	Dprint(2, "MExec: ", arg.Nodes, " fileServer: ", arg.Lfam, arg.Lserver)

	// buffer files on master
	data := bytes.NewBuffer(make([]byte, arg.totalfilebytes))
	Dprint(2, "MExec: doing copyn")	
	n, err := io.Copyn(data, r.ReadWriter(), arg.totalfilebytes)
	Dprint(2, "MExec: copied ", n, " total ", arg.totalfilebytes)
	if err != nil {
		log.Exit("Mexec copyn: ", err)
	}

	/* this is explicitly for sending to remote nodes. So we actually just pick off one node at a time
	 * and call execclient with it. Later we will group nodes.
	 */
	Dprint(2, "MExec nodes ", arg.Nodes)
	for _, n := range arg.Nodes {
		s, ok := Slaves[n]
		Dprintf(2, "node %v == slave %v\n", n, s)
		if !ok {
			log.Printf("No slave %v\n", n)
			continue
		}
		larg := StartArg{
			ThisNode:       true,
			LocalBin:       arg.LocalBin,
			Args:           arg.Args,
			Env:            arg.Env,
			Lfam:           arg.Lfam,
			Lserver:        arg.Lserver,
			cmds:           arg.cmds,
			totalfilebytes: arg.totalfilebytes,
		}

		s.rpc.Send("MExec", larg)
		Dprintf(2, "totalfilebytes %v localbin %v\n", arg.totalfilebytes, arg.LocalBin)
		if arg.LocalBin {
			Dprintf(2, "cmds %v\n", arg.cmds)
		}
		n, err := io.Copyn(s.rpc.ReadWriter(), bytes.NewBuffer(data.Bytes()), arg.totalfilebytes)
		Dprint(2, "MExec wrote ", n)
		if err != nil {
			log.Exit("MExec iocopy: ", err)
		}
		/* at this point it is out of our hands */
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
			log.Exit("masterserve:", err)
		}
		r := NewRpcClientServer(c)
		var a SlaveArg
		r.Recv("masterserve", &a)
		r.Send("masterserve", newSlave(&a, r))
	}
	return nil
}

func newSlave(arg *SlaveArg, r *RpcClientServer) (res SlaveRes) {
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

	Dprintln(2, "Slave id: ", s)
	res.id = s.id
	return
}


/* rewrite this so it uses an interface. This is C code in a Go program. */
func ioreader(w *Worker) {
	data := make([]byte, 1024)
	for {
		n, err := w.Conn.Read(data)
		if n <= 0 {
			break
		}
		if err != nil {
			log.Printf("%s\n", err)
			break
		}

		log.Printf(string(data[0:n]))
	}
	w.Status <- 1
}


