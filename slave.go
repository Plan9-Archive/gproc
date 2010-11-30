package main

import (
	"fmt"
	"log"
	"os"
	"gob"
	"net"
	"syscall"
)


/* the original bproc maintained a persistent connection. That doesn't scale well and, besides,
 * it doesn't fit the RPC model well. So, we're going to set up a server socket and then
 * tell the master about it.
 * we have to connect to a remote, and we have to serve other slaves.
 */
func slave(rfam, raddr string) {
	var ans SlaveRes
	var err os.Error
	a := SlaveArg{id: "-1"}

	client, err := net.Dial(rfam, "", raddr)
	if err != nil {
		log.Exit("dialing:", err)
	}

	e := gob.NewEncoder(client)
	e.Encode(&a)
	d := gob.NewDecoder(client)
	d.Decode(&ans)

	log.Printf("Answer %v\n", ans)
	/* note that we just switched the direction of the
	 * net.Conn. Master is now our client in a sense.
	 * actually once pings go in it's going to be a
	 * bidi show. But we're not sure how we want to do that yet.
	 */
	/* now we just accept commands and do what we need to do */
	for {
		var res Res
		var arg StartArg
		d := gob.NewDecoder(client)
		err = d.Decode(&arg)
		if err != nil {
			break
		}
		/* we've read the StartArg in but not the data.
		 * RExec will ForkExec and do that.
		 */
		RExec(&arg, client, &res)
		e.Encode(&res)
	}
	log.Printf("err %s\n", err)

}

func newSlave(arg *SlaveArg, c net.Conn, res *SlaveRes) os.Error {
	var i int
	var s SlaveInfo
	if arg.id == "-1" {
		i = len(Slaves)
		i++
		s.Addr = arg.a
		s.id = fmt.Sprintf("%d", i)
		res.id = s.id
	} else {
		s = Slaves[arg.id]
		res.id = s.id
		s.Addr = arg.a
	}
	s.client = c
	Slaves[s.id] = s
	fmt.Printf("s is %v\n", s)
	return nil
}


/* rexec will create a listener and then relay the results. We do this go get an IO hierarchy. */
func RExec(arg *StartArg, c net.Conn, res *Res) os.Error {
	if *DebugLevel > 2 {
		log.Printf("Start on nodes %s files call back to %s %s", arg.Nodes, arg.Lfam, arg.Lserver)
	}

	/* set up a pipe */
	r, w, err := os.Pipe()
	defer r.Close()
	defer w.Close()
	if err != nil {
		fmt.Print("Exec:pipe failed: %v\n", err)
	}
	bugger := fmt.Sprintf("-debug=%d", *DebugLevel)
	private := fmt.Sprintf("-p=%v", DoPrivateMount)
	pid, err := os.ForkExec("./gproc", []string{"gproc", bugger, private, "R"}, []string{""}, ".", []*os.File{r, w})
	if *DebugLevel > 2 {
		log.Printf("Forked %d\n", pid)
	}
	if err == nil {
		go func() {
			var status syscall.WaitStatus
			for pid, err := syscall.Wait4(-1, &status, 0, nil); err > 0; pid, err = syscall.Wait4(-1, &status, 0, nil) {
				log.Printf("wait4 returns pid %v status %v\n", pid, status)
			}
		}()
	} else {
		if *DebugLevel > 2 {
			log.Printf("ForkExec failed: %s\n", err)
		}
	}

	/* relay data to the child */
	e := gob.NewEncoder(w)
	if arg.LocalBin && *DebugLevel > 2 {
		log.Printf("RExec arg.LocalBin %v arg.cmds %v\n", arg.LocalBin, arg.cmds)
	}
	e.Encode(arg)
	if *DebugLevel > 2 {
		log.Printf("clone pid %d err %v\n", pid, err)
	}
	b := make([]byte, 8192)
	for i := int64(0); i < arg.totalfilebytes; i += int64(len(b)) {
		amt, err := c.Read(b)
		if amt <= 0 || err != nil {
			log.Panicf("Read from master fails: %\n", err)
		}
		amt, err = w.Write(b[0:amt])
		if amt <= 0 || err != nil {
			log.Panicf("Write to child fails: %\n", err)
		}
	}

	return nil
}
