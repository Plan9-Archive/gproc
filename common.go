/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the GNU Lesser General Public License, version 2, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

package main

import (
	"os"
	"net"
	"fmt"
	"log"
	"io"
	"gob"
	"bitbucket.org/floren/filemarshal"
	"strconv"
	"strings"
	"syscall"
)

type SlaveResp struct {
	Id string
}

func (s SlaveResp) String() string {
	return fmt.Sprint("id: ", s.Id)
}


type Resp struct {
	NumNodes int
	Msg      string
}

func (r Resp) String() string {
	if len(r.Msg) == 0 {
		return "<nil>"
	}
	return string(r.Msg)
}

type SetDebugLevel struct {
	level int
}

type cmdToExec struct {
	Name     string
	FullPath string
	Local    int
	Fi       *os.FileInfo
}

func (a *cmdToExec) String() string {
	return fmt.Sprint(a.Name)
}

/* vitalData is data from the master to the user or slaves to parent (other slaves or master)
 * It can be sent periodically as things change. A slave can inform its parent of new nodes or nodes
 * lost int he Nodes array. Due to the way LocalAddr works, we might as well tell the parent what its 
 * address is ... 
 */

type vitalData struct {
	HostReady  bool
	Error      string
	HostAddr   string
	ParentAddr string
	ServerAddr string
	Nodes      []string
	Exceptlist map[string]bool
}

/* a StartReq is a description of what to run and where to run it.
 * The Nodes are "node numbers" in your "node name space" -- i.e.
 * nodes that have contacted you to tell them who they are.
 * The Peers are "IP address/port" strings from your master
 * that you are told to exec
 * on -- essentially, your master has done the mapping of Nodes to
 * Peers and sent you the raw address information. Peers are used to
 * build the ad-hoc tree.
 * Finally, the ThisBin is a boolean that tells you to run the command
 * yourself. This replaces the bproc "-1" node number which was
 * always a bit of a hack. For now we'll use the -1 numbering
 * for the bpsh command to indicate "local execute" but just
 * set ThisNode in the StartReq when the actual command goes out.
 * This struct is sent, and following it is the data for the files,
 * as a simple stream of bytes.
 * this struct now supports different kinds of commands.
 */
type StartReq struct {
	Command         string
	Nodes           string
	Peers           []string
	ThisNode        bool
	LocalBin        bool
	Args            []string
	Env             []string
	LibList         []string
	Path            string
	Lfam, Lserver   string
	BytesToTransfer int64
	Uid, Gid        int
	Cmds            []*cmdToExec
	/* testing: The master and worker nodes, given a list, will take the head
	 * of the list, and send the rest of the list of Peers on to the next victim. 
	 * this will result in a chain of delegations. 
	 */
	PeerGroupSize int
	Cwd           string
	/* This is where I try the filemarshal thing */
	File []*filemarshal.File
}

func (s *StartReq) String() string {
	return fmt.Sprint(s.Nodes, " ", s.Peers, " ", s.Args, " ", s.Cmds)
}

type Worker struct {
	Alive  bool
	Addr   string
	Conn   net.Conn
	Status chan int
}

type SlaveInfo struct {
	id     string
	Addr   string
	Server string
	Nodes  []string
	rpc    *RpcClientServer
}

func (s *SlaveInfo) String() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprint(s.id, " ", s.Addr)
}


func Dprint(level int, arg ...interface{}) {
	if *DebugLevel >= level {
		log.Print(arg...)
	}
}

func Dprintln(level int, arg ...interface{}) {
	if *DebugLevel >= level {
		log.Println(arg...)
	}
}

func Dprintf(level int, fmt string, arg ...interface{}) {
	if *DebugLevel >= level {
		log.Printf(fmt, arg...)
	}
}

const (
	Send = iota
	Recv
)


func IoString(i interface{}, dir int) string {
	switch i.(type) {
	case net.Conn:
		switch dir {
		case Send:
			return fmt.Sprintf("%8s -> %8s", i.(net.Conn).LocalAddr(), i.(net.Conn).RemoteAddr())
		case Recv:
			return fmt.Sprintf("%8s <- %8s", i.(net.Conn).LocalAddr(), i.(net.Conn).RemoteAddr())
		}
	case *os.File:
		return fmt.Sprint(i.(*os.File).Fd())
	}
	return "<unknown io>"
}

func SendPrint(funcname, to interface{}, arg interface{}) {
	Dprintf(1, "%15s send %25s: %s\n", funcname, IoString(to, Send), arg)
}

func RecvPrint(funcname, from interface{}, arg interface{}) {
	/* works not well. 
	Dprintf(1, "%15s recv %25s: %s\n", funcname, IoString(from, Recv), arg)
	*/
	Dprintf(1, "%v recv %v: %v\n", funcname, from, arg)
}

// this group depends on gob

var roleFunc func(role string)

// no, this is stupid.

type RpcClientServer struct {
	e filemarshal.Encoder
	d filemarshal.Decoder
}

// This is the best way I've come up with to let the slave specify where
// binaries should go.
// You should probably just use *binRoot everywhere here, although it will
// only be used by the slave.
func NewRpcClientServer(rw io.ReadWriter, root string) *RpcClientServer {
	return &RpcClientServer{
		e: filemarshal.NewEncoder(gob.NewEncoder(rw)),
		d: filemarshal.NewDecoder(gob.NewDecoder(rw), root),
	}
}

var onSendFunc func(funcname string, w io.Writer, arg interface{})

func (r *RpcClientServer) Send(funcname string, arg interface{}) {
	SendPrint(funcname, r, arg)
	err := r.e.Encode(arg)
	if err != nil {
		log.Fatal(funcname, ": Send: ", err)
	}
}

var onRecvFunc func(funcname string, r io.Reader, arg interface{})

func (r *RpcClientServer) Recv(funcname string, arg interface{}) {
	err := r.d.Decode(arg)
	if err != nil {
		log.Fatal(funcname, ": Recv error: ", err)
	}
	RecvPrint(funcname, r, arg)
	/* maybe some other time 
	if onRecvFunc != nil {
		onRecvFunc(funcname, r, arg)
	}
	*/
}


var onDialFunc func(fam, laddr, raddr string)

func Dial(fam, laddr, raddr string) (c net.Conn, err os.Error) {
	if onDialFunc != nil {
		onDialFunc(fam, laddr, raddr)
	}
	/* This is terrible, please fix it. Better yet, make the Go guys un-break net.Dial -- John */
	if fam == "tcp" {
		ra, _ := net.ResolveTCPAddr(raddr)
		la, _ := net.ResolveTCPAddr(laddr)
		c, err = net.DialTCP(fam, la, ra)
		if err != nil {
			return
		}
		//Dprint(2, "dial connect ", c.LocalAddr(), "->", c.RemoteAddr())
	} else {
		c, err = net.Dial(fam, raddr)
		//Dprint(2, "dial connect ", c.LocalAddr(), "->", c.RemoteAddr())
	}
	return
}


type Listener struct {
	l net.Listener
}

func (l Listener) Addr() net.Addr {
	return l.l.Addr()
}

var onListenFunc func(fam, laddr string)

func Listen(fam, laddr string) (l Listener, err os.Error) {
	if onListenFunc != nil {
		onListenFunc(fam, laddr)
	}
	ll, err := net.Listen(fam, laddr)
	l.l = ll
	return
}

var onAcceptFunc func(c net.Conn)

func (l Listener) Accept() (c net.Conn, err os.Error) {
	c, err = l.l.Accept()
	if err != nil {
		return
	}
	Dprint(2, "accepted ", c.RemoteAddr(), "->", c.LocalAddr())
	if onAcceptFunc != nil {
		onAcceptFunc(c)
	}
	return
}


// depends on syscall
func WaitAllChildren() {
	var status syscall.WaitStatus
	for {
		pid, err := syscall.Wait4(-1, &status, 0, nil)
		if err <= 0 {
			break
		}
		log.Printf("wait4 returns pid %v status %v\n", pid, status)

	}
}

func newListenProc(jobname string, job func(c *RpcClientServer), srvaddr string) string {
	/* it is important to return the listen address, if this function was called
	 * with port 0
	 */
	netl, err := net.Listen(defaultFam, srvaddr)
	if err != nil {
		log.Fatal("newListenProc: ", err)
	}
	go func() {
		for {
			c, err := netl.Accept()
			if err != nil {
				log.Fatal(jobname, ": ", err)
			}
			Dprint(2, jobname, ": ", c.RemoteAddr())
			go job(NewRpcClientServer(c, *binRoot))
		}
	}()
	return netl.Addr().String()
}

func cacheRelayFilesAndDelegateExec(arg *StartReq, root, Server string) os.Error {
	Dprint(2, "cacheRelayFilesAndDelegateExec: files ", arg.Cmds, " nodes: ", Server, " fileServer: ", arg.Lfam, arg.Lserver)

	larg := newStartReq(arg)

	for _, c := range larg.Cmds {
		Dprint(2, "setupFiles: next cmd: ", c.FullPath)
		if !c.Fi.IsRegular() {
			continue
		}
		fullpath := root + c.FullPath
		file, err := os.Open(fullpath)
		if err != nil {
			log.Printf("Open %v failed: %v\n", fullpath, err)
		}
		file.Close()
	}

	// I don't think this second loop should stick around, but this helps
	// keep it separate from the rest of the old stuff.
	for _, c := range larg.Cmds {
		fullpath := root + c.FullPath
		Dprint(2, "fullpath: ", fullpath)
		f := new(filemarshal.File)
		if c.Fi.IsRegular() || c.Fi.IsDirectory() || c.Fi.IsSymlink() {
			f = &filemarshal.File{Name: c.Name, Fi: *c.Fi, FullPath: c.FullPath}
		} else {
			continue
		}
		larg.File = append(larg.File, f)
	}

	client, err := Dial(defaultFam, "", Server)
	if err != nil {
		log.Print("dialing:", err)
		return err
	}
	Dprintf(2, "connected to %v\n", client)
	rpc := NewRpcClientServer(client, *binRoot)
	Dprintf(2, "rpc client %v, arg %v", rpc, larg)
	go func() {
		rpc.Send("cacheRelayFilesAndDelegateExec", larg)
		Dprintf(2, "bytesToTransfer %v localbin %v\n", arg.BytesToTransfer, arg.LocalBin)
		if arg.LocalBin {
			Dprintf(2, "cmds %v\n", arg.Cmds)
		}
		Dprintf(2, "cacheRelayFilesAndDelegateExec DONE\n")
		/* at this point it is out of our hands */
	}()

	return nil
}

func ioProxy(fam, server string, dest io.Writer) (workerChan chan int, l Listener, err os.Error) {
	workerChan = make(chan int, 0)
	l, err = Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ioproxy: Listen: %v\n", err)
		return
	}
	go func() {
		for whichWorker := 7090; ; whichWorker++ {
			conn, err := l.Accept()
			Dprint(2, "ioProxy: connected by ", conn.RemoteAddr())

			if err != nil {
				Dprint(2, "ioProxy: accept:", err)
				continue
			}
			go func(id int, conn net.Conn) {
				Dprint(2, "ioProxy: start reading ", id)
				n, err := io.Copy(dest, conn)
				workerChan <- id
				Dprint(2, "ioProxy: read ", n)
				if err != nil {
					log.Fatal("ioProxy: ", err)
				}
				Dprint(2, "ioProxy: end")
			}(whichWorker, conn)
		}
	}()
	return
}

type nodeExecList struct {
	Nodes    []string
	Subnodes string
}

/* might be fun to do this as a goroutine feeding a chan of nodeExecList */
/* precedence: comma lowest, then /, then -. I hope. 
 * this makes semi-sensible stuff work. 
 * 1-3/1-3 is all of nodes 1-3 on nodes 1-3. If you want, say, node 1, 2, 
 * and 3/1-3, then write it as 1-2,3/1-3. This form is oriented to what we
 * learned in practice with bproc and xcpu. BTW one thing we learned
 * the really hard way: few people understand regular expressions. So we
 * don't use them. 
 */
func parseNodeList(l string) (rl []nodeExecList, err os.Error) {
	/* bust it apart by , */
	ranges := strings.Split(l, ",", -1)
	for _, n := range ranges {
		/* split into range and rest by the slash */
		l := strings.Split(n, "/", 2)
		be := strings.Split(l[0], "-", 2)
		Dprint(6, " l is ", l, " be is ", be)
		ne := &nodeExecList{Nodes: make([]string, 1)}
		if len(l) > 1 {
			ne.Subnodes = l[1]
		}
		if len(be) == 1 {
			ne.Nodes[0] = be[0]
		} else {
			/* BOGUS! check for bad range here. */
			beg, _ := strconv.Atoi(be[0])
			end, _ := strconv.Atoi(be[1])
			if end < beg {
				goto BadRange
			}
			for i := beg; i <= end; i++ {
				ne.Nodes = append(ne.Nodes, fmt.Sprintf("%d", i))
			}
		}
		rl = append(rl, *ne)
	}

	Dprint(2, "parseNodeList returns ", rl)
	return
BadRange:
	err = BadRangeErr
	return
}

func doPrivateMount(pathbase string) {
	unshare()
	_ = unmount(*binRoot)
	syscallerr := privatemount(*binRoot)
	if syscallerr != 0 {
		log.Print("Mount failed ", syscallerr)
		os.Exit(1)
	}
}

func fileTcpDial(server string) (*os.File, net.Conn, os.Error) {
	var laddr net.TCPAddr
	raddr, err := net.ResolveTCPAddr(server)
	if err != nil {
		return nil, nil, err
	}
	c, err := net.DialTCP(defaultFam, &laddr, raddr)
	if err != nil {
		return nil, nil, err
	}
	f, err := c.File()
	if err != nil {
		c.Close()
		return nil, nil, err
	}

	return f, c, nil
}
