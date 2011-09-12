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
	CurrentName   string
	DestName      string
	SymlinkTarget string
	Local         int
	Fi            *os.FileInfo
}

func (a *cmdToExec) String() string {
	return fmt.Sprint(a.CurrentName)
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
	/* The File element should really replace Cmds */
	Files []*filemarshal.File
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
	Id     string
	Addr   string
	Server string
	Nodes  []string
	Rpc    *RpcClientServer
}

func (s *SlaveInfo) String() string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprint(s.Id, "@", s.Addr)
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

type RpcClientServer struct {
	E filemarshal.Encoder
	D filemarshal.Decoder
}

// This is the best way I've come up with to let the slave specify where
// binaries should go.
// You should probably just use *binRoot everywhere here, although it will
// only be used by the slave.
func NewRpcClientServer(rw io.ReadWriter, root string) *RpcClientServer {
	return &RpcClientServer{
		E: filemarshal.NewEncoder(gob.NewEncoder(rw)),
		D: filemarshal.NewDecoder(gob.NewDecoder(rw), root),
	}
}

var onSendFunc func(funcname string, w io.Writer, arg interface{})

func (r *RpcClientServer) Send(funcname string, arg interface{}) {
	SendPrint(funcname, r, arg)
	err := r.E.Encode(arg)
	if err != nil {
		log.Fatal(funcname, ": Send: ", err)
	}
}

var onRecvFunc func(funcname string, r io.Reader, arg interface{})

func (r *RpcClientServer) Recv(funcname string, arg interface{}) (err os.Error) {
	err = r.D.Decode(arg)
	if err != nil {
		log.Print(funcname, ": Recv error: ", err)
		return
	}
	RecvPrint(funcname, r, arg)
	/* maybe some other time 
	if onRecvFunc != nil {
		onRecvFunc(funcname, r, arg)
	}
	*/
	return
}


var onDialFunc func(fam, laddr, raddr string)

func Dial(fam, laddr, raddr string) (c net.Conn, err os.Error) {
	if onDialFunc != nil {
		onDialFunc(fam, laddr, raddr)
	}
	/* This is terrible, please fix it. Better yet, make the Go guys un-break net.Dial -- John */
	if fam == "tcp" {
		ra, _ := net.ResolveTCPAddr("tcp4", raddr)
		la, _ := net.ResolveTCPAddr("tcp4", laddr)
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

/*
 * This name isn't very good any more.
 * This function builds up a list of files that need to go out to the current node's sub-nodes.
 * It is called by both the master and, if a more complex hierarchy is used, the upper-level slaves.
 */
func cacheRelayFilesAndDelegateExec(arg *StartReq, root, clientnode string) os.Error {
	Dprint(2, "cacheRelayFilesAndDelegateExec: files ", arg.Cmds, " nodes: ", clientnode, " fileServer: ", arg.Lfam, arg.Lserver)

	larg := newStartReq(arg)

	/* Build up a list of filemarshal.File so the filemarshal can transmit the needed files */
	for _, c := range larg.Cmds {
		comesfrom := root + c.DestName
		Dprint(2, "current cmd comesfrom = ", comesfrom, ", DestName = ", c.DestName, ", CurrentName = ", c.CurrentName, ", SymlinkTarget = ", c.SymlinkTarget)
		f := new(filemarshal.File)
		if c.Fi.IsRegular() || c.Fi.IsDirectory() || c.Fi.IsSymlink() {
			f = &filemarshal.File{CurrentName: comesfrom, Fi: *c.Fi, SymlinkTarget: c.SymlinkTarget, DestName: c.DestName}
		} else {
			continue
		}
		larg.Files = append(larg.Files, f)
	}

	client, err := Dial(defaultFam, "", clientnode)
	if err != nil {
		log.Print("cacheRelayFilesAndDelegateExec: dialing: ", clientnode, ": ", err)
		return err
	}
	Dprintf(2, "connected to %v\n", client)
	rpc := NewRpcClientServer(client, *binRoot)
	Dprintf(2, "rpc client %v, arg %v", rpc, larg)
	go func() {
		// This Send pushes our larg struct to filemarshal. Since it contains a
		// []*filemarshal.File, the filemarshal grabs the list of files and sends
		// the file contents too.
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
/*
 * The ioProxy listens for incoming connections. Sub-nodes will connect to it
 * and use the connection as stdin, stdout, and stderr for the programs they
 * execute. ioProxy copies the output of the connection to 'dest', which will
 * be a connection to another ioProxy if we're on a slave or simply stdout
 * if we're in the gproc issuing the exec command.
 * 
 * Whoever calls the ioProxy should read from workerChan to know when I/O is 
 * finished. workerChan will contain one int for every client which has 
 * completed and disconnected.
 */
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
	ranges := strings.SplitN(l, ",", -1)
	for _, n := range ranges {
		/* split into range and rest by the slash */
		l := strings.SplitN(n, "/", 2)
		be := strings.SplitN(l[0], "-", 2)
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

/*
 * Make a TCP connection to a server, return both the connection and
 * a File pointing to that connection.
 */
func fileTcpDial(server string) (*os.File, net.Conn, os.Error) {
	var laddr net.TCPAddr
	raddr, err := net.ResolveTCPAddr("tcp4", server)
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

func newStartReq(arg *StartReq) *StartReq {
	return &StartReq{
		Command:         arg.Command,
		Nodes:           arg.Nodes,
		ThisNode:        true,
		LocalBin:        arg.LocalBin,
		Peers:           arg.Peers,
		Args:            arg.Args,
		Env:             arg.Env,
		LibList:         arg.LibList,
		Path:            arg.Path,
		Lfam:            arg.Lfam,
		Lserver:         arg.Lserver,
		Cmds:            arg.Cmds,
		BytesToTransfer: arg.BytesToTransfer,
		Cwd:             arg.Cwd,
	}
}


/*
 * Functions and data types for keeping track of slave nodes
 */


func registerSlaves(loc Locale) os.Error {
	l, err := Listen(defaultFam, loc.Addr())
	if err != nil {
		log.Fatal("listen error:", err)
	}

	Dprint(0, "-cmdport=", l.Addr())
	Dprint(2, l.Addr())
	err = loc.RegisterServer(l)
	if err != nil {
		log.Fatal(err)
	}

	slaves = newSlaves()
	for {
		vd := &vitalData{}
		c, err := l.Accept()
		if err != nil {
			log.Print("registerSlaves:", err)
			continue
		}
		r := NewRpcClientServer(c, *binRoot)
		if r.Recv("receive vital data", &vd) != nil {
			continue
		}
		/* quite the hack. At some point, on a really complex system, 
		 * we'll need to return a set of listen addresses for a daemon, but we've yet to
		 * see that in actual practice. We can't use LocalAddr here, since it returns our listen
		 * address, not the address we accepted on, and if that's 0.0.0.0, that's useless. 
		 */
		if netaddr == "" {
			addr := strings.SplitN(vd.ParentAddr, ":", 2)
			Dprint(2, "addr is ", addr)
			netaddr = addr[0]
		}
		/* depending on the machine we are on, it is possible we don't get a usable IP address 
		 * in the ServerAddr. We'll have a good port, however, In this case, we need
		 * to cons one up, which is easily done. 
		 */
		if vd.ServerAddr[0:len("0.0.0.0")] == "0.0.0.0" {
			vd.ServerAddr = strings.SplitN(c.RemoteAddr().String(), ":", 2)[0] + vd.ServerAddr[7:]
			Dprint(2, "Guessed remote slave ServerAddr is ", vd.ServerAddr)
		}
		resp := slaves.Add(vd, r)
		r.Send("registerSlaves", resp)
	}
	Dprint(2, "registerSlaves is exiting! That can't be good!")
	return nil
}

type Slaves struct {
	Slaves  map[string]*SlaveInfo
	Addr2id map[string]string
}

func newSlaves() (s Slaves) {
	s.Slaves = make(map[string]*SlaveInfo)
	s.Addr2id = make(map[string]string)
	return
}

func (sv *Slaves) Add(vd *vitalData, r *RpcClientServer) (resp SlaveResp) {
	var s *SlaveInfo
	s = &SlaveInfo{
		Id:     loc.SlaveIdFromVitalData(vd),
		Addr:   vd.HostAddr,
		Server: vd.ServerAddr,
		Nodes:  vd.Nodes,
		Rpc:    r,
	}
	sv.Slaves[s.Id] = s
	sv.Addr2id[s.Server] = s.Id
	Dprintln(2, "slave Add: Id: ", s.Id)
	resp.Id = s.Id
	return
}

func (sv *Slaves) Remove(s *SlaveInfo) {
	Dprintln(4, "Remove %v ", s, " slave %v", sv.Slaves[s.Id])
	sv.Slaves[s.Id] = nil, false
	sv.Addr2id[s.Server] = "", false
	Dprintln(2, "slave Remove: Id: ", s)
	return
}

/* old school: functions with names like GetIP and GetID and so on. 
 * new school: overloading and picking via type signature
 * go school: well, strings are different. So let's try both styles. 
 * one function, but no overloading. Is this good or bad? Who knows? 
 * But the ip and id strings are very, very different, so zero probability
 * of collisions; does that make this ok? 
 */
func (sv *Slaves) Get(n string) (s *SlaveInfo, ok bool) {
	Dprint(6, "Get: ", n)
	s, ok = sv.Slaves[n]
	if ! ok {
		s, ok = sv.Slaves[sv.Addr2id[n]]
	}
	Dprint(6, " Returns: ", s)
	return
}

/* a hack for now. Sorry, we need to clean up the whole parsenodelist/intersect thing
 * but I need something that works and we're still putting the ideas 
 * together. So sue me. 
 */
func (sv *Slaves) ServIntersect(set []string) (i []string) {
	switch set[0] {
	case ".":
		for _, n := range sv.Slaves {
			i = append(i, n.Server)
		}
	default:
		for _, n := range set {
			s, ok := sv.Get(n)
			if !ok {
				continue
			}
			i = append(i, s.Server)
		}
	}
	return
}

var slaves Slaves
