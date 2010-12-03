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

type SlaveRes struct {
	id string
}

func (s SlaveRes) String() string {
	return fmt.Sprint("id: ", s.id)
}


type Res struct {
	Msg []byte
}

func (r Res) String() string {
	if len(r.Msg) == 0 {
		return "<nil>"
	}
	return string(r.Msg)
}


type SlaveArg struct {
	a      string
	id     string
	Msg    []byte
	Server string
}

func (s SlaveArg) String() string {
	if s.id == "" {
		return "<needid>"
	}
	return s.a + " " + s.id + " " + string(s.Msg)
}


type SetDebugLevel struct {
	level int
}

type Acmd struct {
	name         string
	fullpathname string
	local        int
	fi           os.FileInfo
}

func (a Acmd) String() string {
	return fmt.Sprint(a.name)
}


/* a StartArg is a description of what to run and where to run it.
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
 * set ThisNode in the StartArg when the actual command goes out.
 * This struct is sent, and following it is the data for the files,
 * as a simple stream of bytes.
 */
type StartArg struct {
	Nodes          []string
	Peers          []string
	ThisNode       bool
	LocalBin       bool
	Args           []string
	Env            []string
	Lfam, Lserver  string
	totalfilebytes int64
	uid, gid       int
	cmds           []Acmd
}

func (s *StartArg) String() string {
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
	client net.Conn
}

func (s *SlaveInfo) String() string {
	if s != nil {
		return "<nil>"
	}
	return fmt.Sprint(s.id, " ", s.Addr, " ", s.client)
}

var Slaves map[string]SlaveInfo

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

func IoString(i interface{}) string {
	switch i.(type) {
	case net.Conn:
		return fmt.Sprint(i.(net.Conn).RemoteAddr())
	case *os.File:
		return fmt.Sprint(i.(*os.File).Fd())
	}
	return "<unknown io>"
}

func SendPrint(funcname, to interface{}, arg interface{}) {
	Dprint(1, "		", funcname, ": send 				", IoString(to), ":		", arg)
}

func RecvPrint(funcname, from interface{}, arg interface{}) {
	Dprint(1, "		", funcname, ": recv		", IoString(from), ":		", arg)
}

// depends on gob
func Send(funcname string, w io.Writer, arg interface{}) {
	e := gob.NewEncoder(w)
	SendPrint(funcname, w, arg)
	err := e.Encode(arg)
	if err != nil {
		log.Exit(funcname, ": ", err)
	}
}

func Recv(funcname string, r io.Reader, arg interface{}) {
	e := gob.NewDecoder(r)
	RecvPrint(funcname, r, arg)
	err := e.Decode(arg)
	if err != nil {
		log.Exit(funcname, ": ", err)
	}
}

// depends on syscall
func Wait4() {
	var status syscall.WaitStatus
	for pid, err := syscall.Wait4(-1, &status, 0, nil); err > 0; pid, err = syscall.Wait4(-1, &status, 0, nil) {
		log.Printf("wait4 returns pid %v status %v\n", pid, status)
	}
}

func newListenProc(jobname string, job func(c net.Conn), srvaddr string) {
	netl, err := net.Listen("tcp4", srvaddr)
	if err != nil {
		log.Exit("newListenProc: ", err)
	}
	go func() {
		c, err := netl.Accept()
		if err != nil {
			log.Exit(jobname, ": ", err)
		}
		Dprint(2, jobname, ": ", c.RemoteAddr())
		go job(c)
	}()
}
