package main

import (
	"log"
	"os"
	"net"
	"rpc"
	"fmt"
	"syscall"
	"strconv"
	"strings"
	"container/vector"
	"flag"
	"json"
	"io/ioutil"
	"bitbucket.org/npe/ldd"
)

type Arg struct {
	Msg []byte
}

type Res struct {
	Msg []byte
}

type SlaveArg struct {
	a   string
	id  string
	Msg []byte
}

type SlaveRes struct {
	id string
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

type noderange struct {
	Base int
	Ip   string
}

type gpconfig struct {
	Noderanges []noderange
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

type SlaveInfo struct {
	id     string
	Addr   string
	client net.Conn
}

type Worker struct {
	Alive  bool
	Addr   string
	Conn   net.Conn
	Status chan int
}

func usage() {
	fmt.Fprint(os.Stderr, "usage: gproc m <path>\n")
	fmt.Fprint(os.Stderr, "usage: gproc s <family> <address>\n")
	fmt.Fprint(os.Stderr, "usage: gproc e <server address> <fam> <address> <nodes> <command>\n")
	fmt.Fprint(os.Stderr, "usage: gproc R\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	Logfile = "/tmp/log"
	Slaves  map[string]SlaveInfo
	Workers []Worker

	localbin       = flag.Bool("localbin", false, "execute local files")
	DoPrivateMount = flag.Bool("p", true, "Do a private mount")
	DebugLevel     = flag.Int("debug", 0, "debug level")
	/* this one gets me a zero-length string if not set. Phooey. */
	takeout = flag.String("f", "", "comma-seperated list of files/directories to take along")
	root    = flag.String("r", "", "root for finding binaries")
	libs    = flag.String("L", "/lib:/usr/lib", "library path")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	Slaves = make(map[string]SlaveInfo, 1024)
	config := getConfig()
	if *DebugLevel > -1 {
		log.Printf("config is %v\n", config)
		log.Printf("gproc starts with %v and DebugLevel is %d\n", os.Args, *DebugLevel)
	}
	switch flag.Arg(0) {
	/* traditional bproc master, commands over unix domain socket */
	case "d":
		SetDebugLevelRPC(flag.Arg(1), flag.Arg(2), flag.Arg(3))
	case "m":
		if len(flag.Args()) < 2 {
			flag.Usage()
		}
		master(flag.Arg(1))
	case "s":
		/* traditional slave; connect to master, await instructions */
		if len(flag.Args()) < 3 {
			flag.Usage()
		}
		slave(flag.Arg(1), flag.Arg(2))
	case "e":
		if len(flag.Args()) < 6 {
			flag.Usage()
		}
		mexec()
	case "R":
		run()
	default:
		flag.Usage()
	}
}

func setupLog() {
	logfile, err := os.Open(Logfile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Panic("No log file", err)
	}
	log.SetOutput(logfile)
	log.Printf("DoPrivateMount: %v\n", DoPrivateMount)

}

func notslash(c int) bool {
	if c != '/' {
		return true
	}
	return false
}

func slash(c int) bool {
	if c == '/' {
		return true
	}
	return false
}

func packdir(l string, flist *vector.Vector, dodir bool) os.Error {
	f, err := os.Open(l, 0, 0)
	if err != nil {
		return err
	}
	list, err := f.Readdirnames(-1)

	if err != nil {
		return err
	}

	for _, s := range list {
		packfile(l+"/"+s, "", flist, false)
	}
	return nil
}
/* we do the files here. We push the files and then the directories. We just push them on,
 * duplicates and all, and do the dedup later when we pop them.
 */
func packfile(l, root string, flist *vector.Vector, dodir bool) os.Error {
	/* what a hack we need to fix this idiocy */
	if len(l) < 1 {
		return nil
	}
	_, err := os.Stat(root + l)
	if err != nil {
		log.Panic("Bad file: ", root+l, err)
		return err
	}
	/* Push the file, then its components. Then we pop and get it all back in the right order */
	curfile := l
	for len(curfile) > 0 {
		fi, _ := os.Stat(root + curfile)
		/* if it is a directory, and we're following them, do the elements first. */
		if dodir && fi.IsDirectory() {
			packdir(curfile, flist, false)
		}
		c := Acmd{curfile, root + curfile, 0, *fi}
		if *DebugLevel > 2 {
			log.Printf("Push %v stat %v\n", c.name, fi)
		}
		flist.Push(&c)
		curfile = strings.TrimRightFunc(curfile, notslash)
		curfile = strings.TrimRightFunc(curfile, slash)
		/* we don't dodir on our parents. */
		dodir = false
	}
	return nil
}

func RangeList(l string) []string {
	var ret []string
	ll := strings.Split(l, "-", -1)
	switch len(ll) {
	case 2:
		var start, end int
		cnt, err := fmt.Sscanf(ll[0], "%d", &start)
		if cnt != 1 || err != nil {
			fmt.Printf("Bad number: %v\n", ll[0])
		}
		cnt, err = fmt.Sscanf(ll[1], "%d", &end)
		if cnt != 1 || err != nil {
			fmt.Printf("Bad number: %v\n", ll[1])
		}
		if start > end {
			fmt.Printf("%d > %d\n", start, end)
		}
		ret = make([]string, end-start+1)
		for i := start; i <= end; i++ {
			ret[i-start] = fmt.Sprint(i)
		}
	case 1:
		ret = ll
	default:
		fmt.Print("%s: bogus\n", l)
		return nil
	}
	return ret
}
func NodeList(l string) []string {
	var ret []string
	l = strings.Trim(l, " ,")
	ll := strings.Split(l, ",", -1)

	for _, s := range ll {
		newlist := RangeList(s)
		if newlist == nil {
			continue
		}
		nextret := make([]string, len(ret)+len(newlist))
		if ret != nil {
			copy(nextret[0:], ret[0:])
		}
		copy(nextret[len(ret):], newlist[0:])
		ret = nextret
	}
	return ret
}

func Ping(arg *Arg, res *Res) os.Error {
	res.Msg = arg.Msg
	return nil
}

func Debug(arg *SetDebugLevel, res *SetDebugLevel) os.Error {
	res.level = *DebugLevel
	*DebugLevel = arg.level
	return nil
}



func readitin(s, root string) ([]byte, os.FileInfo, os.Error) {
	fi, _ := os.Stat(root + s)
	f, _ := os.Open(s, os.O_RDONLY, 0)
	bytes := make([]byte, fi.Size)
	f.Read(bytes)
	return bytes, *fi, nil
}


func writeitout(in *os.File, s string, fi os.FileInfo) (int, os.Error) {
	var err os.Error
	var filelen int = 0
	out := "/tmp/xproc" + s
	if *DebugLevel > 2 {
		log.Printf("write out  %s, %v %v\n", out, fi, fi.Mode)
	}
	switch fi.Mode & syscall.S_IFMT {
	case syscall.S_IFDIR:
		err = os.Mkdir(out, fi.Mode&0777)
		if err != nil {
			err = os.Chown(out, fi.Uid, fi.Gid)
		}
	case syscall.S_IFLNK:
		err = os.Symlink(out, "/tmp/xproc"+fi.Name)
	case syscall.S_IFREG:
		f, err := os.Open(out, os.O_RDWR|os.O_CREAT, 0777)
		if err != nil {
			return -1, err
		}
		defer f.Close()
		b := make([]byte, 8192)
		for i := int64(0); i < fi.Size; {
			var amt int = int(fi.Size - i)
			if amt > len(b) {
				amt = len(b)
			}
			amt, _ = in.Read(b[0:amt])
			amt, err = f.Write(b[0:amt])
			if err != nil {
				return -1, err
			}
			i += int64(amt)
			if *DebugLevel > 5 {
				log.Printf("Processed %d of %d\n", i, fi.Size)
			}
		}
		if *DebugLevel > 5 {
			log.Printf("Done %v\n", out)
		}
		if err != nil {
			err = os.Chown(out, fi.Uid, fi.Gid)
		}
	default:
		return -1, nil
	}

	if *DebugLevel > 2 {
		log.Printf("Finished %v\n", out)
	}
	return filelen, nil
}


func netrelay(c net.Conn, workers chan int, client net.Conn) {
	data := make([]byte, 1024)
	for {
		n, _ := c.Read(data)
		if n <= 0 {
			break
		}
		amt, err := client.Write(data)
		if amt <= 0 {
			log.Printf("Write failed: amt %d, err %v\n", amt, err)
			break
		}
		if err != nil {
			log.Printf("Write failed: %v\n", err)
			break
		}
	}
	workers <- 1
}

func netwaiter(fam, server string, nw int, c net.Conn) (chan int, net.Listener) {
	workers := make(chan int, nw)
	l, err := net.Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %s\n", err)
		return nil, nil
	}

	go func() {
		for ; nw > 0; nw-- {
			conn, err := l.Accept()
			if err != nil {
				log.Printf("%s\n", err)
				continue
			}
			go netrelay(conn, workers, c)
		}
	}()
	return workers, l
}


/* let's be nice and do an Ldd on each file. That's helpful to people. Later. */
func buildcmds(file, root, libs string) []Acmd {
	e, _ := ldd.Ldd(file, root, libs)
	/* now we have a list of file names. From this, we create the in-memory
	 * packed set of files/symlinks/directory descriptions. We also need to track
	 * what weve made and might have made earlier, to avoid duplicates.
	 */
	cmds := make([]Acmd, len(e))
	for i, s := range e {
		cmds[i].name = s
		cmds[i].fullpathname = root + s
		fi, _ := os.Stat(root + s)
		cmds[i].fi = *fi
	}
	return cmds
}


func SetDebugLevelRPC(fam, server, newlevel string) {
	var ans SetDebugLevel
	level, err := strconv.Atoi(newlevel)
	if err != nil {
		log.Exit("bad level:", err)
	}

	a := SetDebugLevel{level} // Synchronous call
	client, err := rpc.DialHTTP(fam, server)
	if err != nil {
		log.Exit("dialing:", err)
	}
	err = client.Call("Node.Debug", a, &ans)
	if err != nil {
		log.Exit("error:", err)
	}
	log.Printf("Was %d is %d\n", ans.level, level)

}


func getConfig() (config gpconfig) {
	for _, s := range []string{"gpconfig", "/etc/clustermatic/gpconfig"} {
		configdata, _ := ioutil.ReadFile(s)
		if configdata == nil {
			continue
		}
		err := json.Unmarshal(configdata, &config)
		if err != nil {
			fmt.Printf("Bad config file: %v\n", err)
			os.Exit(1)
		}
		break
	}
	return
}
