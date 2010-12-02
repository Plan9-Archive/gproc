package main

import (
	"os"
	"net"
	"fmt"
	"log"
)

type SlaveRes struct {
	id string
}

func (s SlaveRes) String() string {
	return fmt.Sprint("id", s.id)
}


type Res struct {
	Msg []byte
}

type SlaveArg struct {
	a   string
	id  string
	Msg []byte
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
	return fmt.Sprint(s.Nodes," ",s.Peers," ",s.Args," ",s.cmds)	
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
	client net.Conn
}

func (s *SlaveInfo) String() string {
	if s != nil {
		return "<nil>"
	}
	return fmt.Sprint(s.id," ",s.Addr," ",s.client)
}

var Slaves  map[string]SlaveInfo

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

