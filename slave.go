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

	client, err := net.Dial(rfam, "", raddr)
	if err != nil {
		log.Exit("dialing:", err)
	}

	e := gob.NewEncoder(client)
	a := new(SlaveArg)
	err = e.Encode(&a)
	if err != nil {
		log.Exit(err)
	}
	log.SetPrefix(a.id+": ")
	d := gob.NewDecoder(client)
	err = d.Decode(&ans)
	if err != nil {
		log.Exit(err)
	}
	Dprint(2, "Answer", ans)
	/* note that we just switched the direction of the
	 * net.Conn. Master is now our client in a sense.
	 * actually once pings go in it's going to be a
	 * bidi show. But we're not sure how we want to do that yet.
	 */
	/* now we just accept commands and do what we need to do */
	for {
		d := gob.NewDecoder(client)
		arg := new(StartArg)
		err = d.Decode(arg)
		if err != nil {
			break
		}
		if *DebugLevel > 2 {
			log.Println("slave: arg", arg)
		}
		/* we've read the StartArg in but not the data.
		 * RExec will ForkExec and do that.
		 */
		res := new(Res)
		RExec(arg, client, res)
		if *DebugLevel > 2 {
			log.Println("slave: res", res)
		}
		e.Encode(&res)
	}
	log.Exit(err)
}


/* rexec will create a listener and then relay the results. We do this go get an IO hierarchy. */
func RExec(arg *StartArg, c net.Conn, res *Res) os.Error {
	Dprintln(2, "RExec:", arg.Nodes, "fileServer", arg)
	/* set up a pipe */
	r, w, err := os.Pipe()
	defer r.Close()
	defer w.Close()
	if err != nil {
		log.Exit("Exec:", err)
	}
	bugger := fmt.Sprintf("-debug=%d", *DebugLevel)
	private := fmt.Sprintf("-p=%v", DoPrivateMount)
	pid, err := os.ForkExec("./gproc", []string{"gproc", bugger, private, "R"}, []string{""}, ".", []*os.File{r, w})
	if err != nil {
		log.Exit(err)
	}
	Dprintf(2, "Forked %d\n", pid)
	go func() {
		var status syscall.WaitStatus
		for pid, errno := syscall.Wait4(-1, &status, 0, nil); errno > 0; pid, errno = syscall.Wait4(-1, &status, 0, nil) {
			if errno != 0 {
				log.Exit(err)
			}
			log.Printf("wait4 returns pid %v status %v\n", pid, status)
		}
	}()

	/* relay data to the child */
	e := gob.NewEncoder(w)
	if arg.LocalBin {
		Dprintf(2, "RExec arg.LocalBin %v arg.cmds %v\n", arg.LocalBin, arg.cmds)
	}
	err = e.Encode(arg)
	if err != nil {
		log.Exit(err)
	}
	Dprintf(2, "clone pid %d err %v\n", pid, err)
	b := make([]byte, 8192)
	for i := int64(0); i < arg.totalfilebytes; i += int64(len(b)) {
		amt, err := c.Read(b)
		if amt <= 0 || err != nil {
			log.Exitf("Read from master fails: %\n", err)
		}
		amt, err = w.Write(b[0:amt])
		if amt <= 0 || err != nil {
			log.Exitf("Write to child fails: %\n", err)
		}
	}

	return nil
}
