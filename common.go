package main

import (
	"os"
	"net"
	"fmt"
	"log"
	"io"
	"gob"
	"syscall"
)

type SlaveResp struct {
	id string
}

func (s SlaveResp) String() string {
	return fmt.Sprint("id: ", s.id)
}


type Resp struct {
	Msg []byte
}

func (r Resp) String() string {
	if len(r.Msg) == 0 {
		return "<nil>"
	}
	return string(r.Msg)
}


type SlaveReq struct {
	a      string
	id     string
	Msg    []byte
	Server string
}

func (s SlaveReq) String() string {
	if s.id == "" {
		return "<needid>"
	}
	return s.a + " " + s.id + " " + string(s.Msg)
}


type SetDebugLevel struct {
	level int
}

type cmdToExec struct {
	name         string
	fullPath string
	local        int
	fi           *os.FileInfo
}

func (a *cmdToExec) String() string {
	return fmt.Sprint(a.name)
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
 */
type StartReq struct {
	Nodes          []string
	Peers          []string
	ThisNode       bool
	LocalBin       bool
	Args           []string
	Env            []string
	Lfam, Lserver  string
	bytesToTransfer int64
	uid, gid       int
	cmds           []*cmdToExec
	/* testing: The master and worker nodes, given a list, will take the head
	 * of the list, and send the rest of the list of Peers on to the next victim. 
	 * this will result in a chain of delegations. 
	 */
	chainWorkers int
}

func (s *StartReq) String() string {
	return fmt.Sprint(s.Nodes, " ", s.Peers, " ", s.Args, " ", s.cmds)
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
	Dprintf(1, "%15s recv %25s: %s\n", funcname, IoString(from, Recv), arg)
}

// this group depends on gob

var roleFunc func(role string)

// no, this is stupid.

type RpcClientServer struct {
	rw io.ReadWriter
	e  *gob.Encoder
	d  *gob.Decoder
}

func NewRpcClientServer(rw io.ReadWriter) *RpcClientServer {
	return &RpcClientServer{
		rw: rw,
		e:  gob.NewEncoder(rw),
		d:  gob.NewDecoder(rw),
	}
}

func (r *RpcClientServer) ReadWriter() io.ReadWriter {
	return r.rw
}

var onSendFunc func(funcname string, w io.Writer, arg interface{})

func (r *RpcClientServer) Send(funcname string, arg interface{}) {
	SendPrint(funcname, r.rw, arg)
	err := r.e.Encode(arg)
	if err != nil {
		log.Exit(funcname, ": Send: ", err)
	}
}

var onRecvFunc func(funcname string, r io.Reader, arg interface{})

func (r *RpcClientServer) Recv(funcname string, arg interface{}) {
	err := r.d.Decode(arg)
	if err != nil {
		log.Exit(funcname, ": Recv: ", err)
	}
	RecvPrint(funcname, r.rw, arg)
	if onRecvFunc != nil {
		onRecvFunc(funcname, r.rw, arg)
	}
}

var onDialFunc func(fam, laddr, raddr string)

func Dial(fam, laddr, raddr string) (c net.Conn, err os.Error) {
	if onDialFunc != nil {
		onDialFunc(fam, laddr, raddr)
	}
	c, err = net.Dial(fam, laddr, raddr)
	if err != nil {
		return
	}
	Dprint(2, "dial connect ", c.LocalAddr(),"->", c.RemoteAddr())
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
	netl, err := net.Listen("tcp4", srvaddr)
	if err != nil {
		log.Exit("newListenProc: ", err)
	}
	go func() {
		for {
			c, err := netl.Accept()
			if err != nil {
				log.Exit(jobname, ": ", err)
			}
			Dprint(2, jobname, ": ", c.RemoteAddr())
			go job(NewRpcClientServer(c))
		}
	}()
	return netl.Addr().String()
}
