package main

import (
	"os"
	"net"
	"log"
	"gob"
	"fmt"
	"io/ioutil"
)

var Workers []Worker

/* the most complex one. Needs to ForkExec itself, after
 * pasting the fd for the accept over the stdin etc.
 * and the complication of course is that net.Conn is
 * not able to do this, we have to relay the data
 * via a pipe. Oh well, at least we get to manage the
 * net.Conn without worrying about child fooling with it. BLEAH.
 */
func master(addr string) {
	Dprintln(2,"starting master")
	l, err := net.Listen("unix", addr)
	if err != nil {
		log.Exit("listen error:", err)
	}

	go unixserve(l)

	netl, err := net.Listen("tcp4", "0.0.0.0:0")
	if err != nil {
		log.Exit("listen error:", err)
	}
	log.SetPrefix("master:")
	Dprint(2,"master:", netl.Addr())
	fmt.Println(netl.Addr())
	err = ioutil.WriteFile("/tmp/srvaddr", []byte(netl.Addr().String()), 0644)
	if err != nil {
		log.Exit(err)
	}
	
	masterserve(netl)

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
		c, err := l.Accept()
		if err != nil {
			log.Exit("masterserve:", err)
		}

		d := gob.NewDecoder(c)
		d.Decode(&s)
		if err != nil {
			log.Exit(err)
		}
		newSlave(&s, c, &r)
		e := gob.NewEncoder(c)
		err = e.Encode(&r)
		if err != nil {
			log.Exit(err)
		}
	}
	return nil
}

func newSlave(arg *SlaveArg, c net.Conn, res *SlaveRes) os.Error {
	var i int
	var s SlaveInfo
	if arg.id == "" {
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
	Dprintln(2,"Slaves[s.id]:", s)
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

		log.Printf(string(data[0:n]))
	}
	w.Status <- 1
}

func MExec(arg *StartArg, c net.Conn) os.Error {
	if *DebugLevel > 2 {
		log.Print("MExec: ",arg.Nodes, " fileServer: ", arg.Lfam, arg.Lserver)
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
			log.Exitf("transfer read: %v: %v\n", in, err)
		}
		amt, err = out.Write(b[0:amt])
		if err != nil {
			log.Exitf("transfer read: %v", err)
		}
		if amt == 0 {
			log.Exitln("0 byte write!")
			return nil
		}
		i += amt
	}
	return nil
}



