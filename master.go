package main

import (
	"os"
	"net"
	"log"
	"gob"
	"fmt"
)

func MExec(arg *StartArg, c net.Conn) os.Error {
	if *DebugLevel > 2 {
		fmt.Fprintf(os.Stderr, "Start on nodes %s files call back to %s %s", arg.Nodes, arg.Lfam, arg.Lserver)
	}

	/* suck in all the file data. Only the master need do this. */
	data := make([]byte, arg.totalfilebytes)
	for i := int64(0); i < arg.totalfilebytes; {
		amt, err := c.Read(data[i:])
		if err != nil {
			log.Printf("Read error %v: Giving up\n", err)
			return err
		}
		i += int64(amt)
	}
	/* this is explicitly for sending to remote nodes. So we actually just pick off one node at a time
	 * and call execclient with it. Later we will group nodes.
	 */
	for _, n := range arg.Nodes {
		s, ok := Slaves[n]
		if *DebugLevel > 2 {
			log.Printf("Node %v is slave %v\n", n, s)
		}
		if !ok {
			log.Printf("No slave %v\n", n)
			continue
		}
		larg := StartArg{ThisNode: true, LocalBin: arg.LocalBin, Args: arg.Args, Env: arg.Env, Lfam: arg.Lfam, Lserver: arg.Lserver, cmds: arg.cmds, totalfilebytes: arg.totalfilebytes}
		e := gob.NewEncoder(s.client)
		err := e.Encode(larg)
		if err != nil {
			log.Printf("Encode error on s %v: he's dead jim\n", s)
			continue
		}
		if *DebugLevel > 2 {
			log.Printf("totalfilebytes %v localbin %v\n", arg.totalfilebytes, arg.LocalBin)
		}
		if arg.LocalBin && *DebugLevel > 2 {
			log.Printf("cmds %v\n", arg.cmds)
		}
		for i := int64(0); i < arg.totalfilebytes; {
			actual, err := s.client.Write(data[i:])
			i += int64(actual)
			if err != nil {
				log.Printf("Write to slave %s failed: %v", s, err)
				break
			}
		}
		/* at this point it is out of our hands */
	}
	res := Res{Msg: []byte("Message: I care")}
	e := gob.NewEncoder(c)
	e.Encode(res)
	return nil
}

func transfer(in *os.File, out net.Conn, length int) os.Error {
	var err os.Error
	b := make([]byte, 8192)
	var amt int
	for i := 0; i < length; {
		amt, err = in.Read(b)
		if err != nil {
			log.Panicf("transfer read: %v: %v\n", in, err)
		}
		amt, err = out.Write(b[0:amt])
		if err != nil {
			log.Panic("transfer read: %v", err)
		}
		if amt == 0 {
			log.Panic("0 byte write!\n")
			return nil
		}
		i += amt
	}
	return nil
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

		fmt.Printf(string(data[0:n]))
	}
	w.Status <- 1
}

func unixserve(l net.Listener) os.Error {
	for {
		var a StartArg
		c, err := l.Accept()
		if err != nil {
			log.Printf("unixserve: accept on (%v) failed %v\n", l, err)
		}
		go func() {
			d := gob.NewDecoder(c)
			d.Decode(&a)
			/*
				_, uid, gid := ucred(0)
				a.uid = uid
				a.gid = gid
			*/
			MExec(&a, c)
		}()
	}
	return nil
}

/* you need to keep making new encode/decoders because the process
 * at the other end is always new
 */
func masterserve(l net.Listener) os.Error {
	for {
		var s SlaveArg
		var r SlaveRes
		c, _ := l.Accept()
		d := gob.NewDecoder(c)
		d.Decode(&s)
		newSlave(&s, c, &r)
		e := gob.NewEncoder(c)
		e.Encode(&r)
	}
	return nil
}

/* the most complex one. Needs to ForkExec itself, after
 * pasting the fd for the accept over the stdin etc.
 * and the complication of course is that net.Conn is
 * not able to do this, we have to relay the data
 * via a pipe. Oh well, at least we get to manage the
 * net.Conn without worrying about child fooling with it. BLEAH.
 */
func master(addr string) {
	l, e := net.Listen("unix", addr)
	if e != nil {
		log.Exit("listen error:", e)
	}

	go unixserve(l)

	netl, e := net.Listen("tcp4", "0.0.0.0:0")
	if e != nil {
		log.Exit("listen error:", e)
	}
	fmt.Printf("Serving on %v\n", netl.Addr())

	masterserve(netl)

}

func iowaiter(fam, server string, nw int) (chan int, net.Listener) {
	workers := make(chan int, nw)
	Workers := make([]*Worker, nw)
	l, err := net.Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %v\n", err)
		return nil, nil
	}

	go func() {
		for i := 0; nw > 0; nw, i = nw-1, i+1 {
			conn, err := l.Accept()
			w := &Worker{Alive: true, Conn: conn, Status: workers}
			Workers[i] = w
			if err != nil {
				log.Printf("%v\n", err)
				continue
			}
			go ioreader(w)
		}
	}()
	return workers, l
}
