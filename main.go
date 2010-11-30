package main

import (
	"log"
	"os"
	"net"
	"rpc"
	"fmt"
	"bitbucket.org/npe/ldd"
	"syscall"
	"strconv"
	"strings"
	"container/vector"
	"gob"
	"flag"
	"json"
	"io/ioutil"
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
	name  string
	fullpathname string
	local int
	fi    os.FileInfo
}

type noderange struct {
	Base int
	Ip string
}

type gpconfig struct {
	Noderanges [ ]noderange
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
	Alive bool
	Addr string
	Conn net.Conn
	Status chan int
}

func usage() {
	fmt.Fprint(os.Stderr, "usage: gproc m <path>\n")
	fmt.Fprint(os.Stderr, "usage: gproc s <family> <address>\n")
	fmt.Fprint(os.Stderr, "usage: gproc e <server address> <fam> <address> <nodes> <command>\n")
	fmt.Fprint(os.Stderr, "usage: gproc R <cmd>\n")
	flag.PrintDefaults()
	os.Exit(2)
}

var (
	localbin = flag.Bool("localbin", false, "execute local files")
	DoPrivateMount = flag.Bool("p", true, "Do a private mount")
	DebugLevel = flag.Int("debug", 0, "debug level")
	/* this one gets me a zero-length string if not set. Phooey. */
	takeout = flag.String("f", "", "comma-seperated list of files/directories to take along")
	root = flag.String("r", "", "root for finding binaries")
	libs = flag.String("L", "/lib:/usr/lib", "library path")
)
var Logfile = "/tmp/log"
var Slaves map[string]SlaveInfo
var Workers []Worker

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
		log.Panic("Bad file: ", root + l, err)
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
		c := Acmd{curfile, root+curfile, 0, *fi}
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

/* started by gproc. Data comes in on stdin. We create the
 * whole file tree in a private name space -- this is
 * to keep the process image from growing too big.
 * we almost certainly exec it. Then we send all those
 * files right back out again to other nodes if needed
 * (later).
 * We always make and mount /tmp/xproc, and chdir to it, so the
 * programs have a safe place to stash files that might go away after
 * all is done.
 * Due to memory footprint issues, we really can not have both the
 * files and a copy of the data in memory. (the files are in ram too).
 * So this function is responsible for issuing the commands to our
 * peerlist as well as to any subnodes. We run a goroutine for
 * each peer and mexecclient for the children.
 */
func run() {
	var arg StartArg
	var pathbase = "/tmp/xproc"
	d := gob.NewDecoder(os.Stdin)
	d.Decode(&arg)
	/* make sure the directory exists and then do the private name space mount */

	if *DebugLevel > 3 {
		log.Printf("arg is %v\n", arg)
	}
	os.Mkdir(pathbase, 0700)
	if *DoPrivateMount == true {
		unshare()
		_ = unmount(pathbase)
		syscallerr := privatemount(pathbase)
		if syscallerr != 0 {
			log.Printf("Mount failed", syscallerr, "\n")
			os.Exit(1)
		}
	}

	for _, s := range arg.cmds {
		if *DebugLevel > 2 {
			log.Printf("Localbin %v cmd %v:", arg.LocalBin, s)
			log.Printf("%s\n", s.name)
		}
		_, err := writeitout(os.Stdin, s.name, s.fi)
		if err != nil {
			break
		}
	}
	if *DebugLevel > 2 {
		log.Printf("Connect to %v\n", arg.Lserver)
	}

	sock := connect(arg.Lserver)

	if sock < 0 {
		os.Exit(1)
	}
	n := os.NewFile(sock, "child_process_socket")
	f := []*os.File{n, n, n}
	execpath := pathbase + arg.Args[0]
	if arg.LocalBin {
		execpath = arg.Args[0]
	}
	_, err := os.ForkExec(execpath, arg.Args, arg.Env, pathbase, f)
	n.Close()
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
	os.Exit(1)
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

func readitin(s, root string) ([]byte, os.FileInfo, os.Error) {
	fi, _ := os.Stat(root + s)
	f, _ := os.Open(s, os.O_RDONLY, 0)
	bytes := make([]byte, fi.Size)
	f.Read(bytes)
	return bytes, *fi, nil
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

func iowaiter(fam, server string, nw int) (chan int, net.Listener) {
	workers := make(chan int, nw)
	Workers := make([]*Worker, nw)
	l, err := net.Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %v\n", err)
		return nil, nil
	}

	go func() {
		for i := 0; nw > 0; nw, i = nw - 1, i + 1 {
			conn, err := l.Accept()
			w := &Worker{Alive:true, Conn: conn, Status: workers}
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

/* Ask the master for it.
 */
func mexecclient(fam, server string, nodes, peers []string, cmds []Acmd, args []string, l net.Listener, workers chan int) os.Error {
	nworkers := len(nodes) + len(peers)
	var ans Res
	var err os.Error
	a := StartArg{Lfam: string(l.Addr().Network()), Lserver: string(l.Addr().String()), cmds: nil, LocalBin: *localbin}
	files := make([]*os.File, len(cmds))
	for i := 0; i < len(cmds); i++ {
		if *DebugLevel > 2 {
			fmt.Printf("cmd %v\n", cmds[i])
		}
		if !cmds[i].fi.IsRegular() {
			continue
		}
		files[i], err = os.Open(cmds[i].fullpathname, os.O_RDONLY, 0)
		if err != nil {
			fmt.Printf("Open %v failed: %v\n", cmds[i].fullpathname, err)
		}
		defer files[i].Close()
		a.totalfilebytes += cmds[i].fi.Size
	}
	if *DebugLevel > 2 {
		log.Printf("Total file bytes: %v\n", a.totalfilebytes)
	}
	a.Args = make([]string, 1)
	a.Args = args
	a.Env = make([]string, 1)
	a.Env[0] = "LD_LIBRARY_PATH=/tmp/xproc/lib:/tmp/xproc/lib64"
	a.Nodes = make([]string, len(nodes))
	a.Nodes = nodes
	a.cmds = cmds
	client, err := net.Dial(fam, "", server)
	if err != nil {
		log.Exit("dialing:", fam, server, err)
	}

	e := gob.NewEncoder(client)
	e.Encode(&a)

	if err != nil {
		log.Exit("error:", err)
	}

	for i := 0; i < len(files); i++ {
		if !cmds[i].fi.IsRegular() {
			continue
		}
		err = transfer(files[i], client, int(cmds[i].fi.Size))
		if err != nil {
			return nil
		}
	}
	d := gob.NewDecoder(client)
	d.Decode(&ans)

	for ; nworkers > 0; nworkers-- {
		<-workers
	}
	return nil
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
		} ()
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




func main() {
	var takeout, root, libs string
	var config gpconfig
	Slaves = make(map[string]SlaveInfo, 1024)
	flag.Usage = usage
	flag.Parse()

	/* do the Config thing */
	for _,s := range []string{"gpconfig", "/etc/clustermatic/gpconfig"} {
		configdata,_ := ioutil.ReadFile(s)
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
	/*
		if len(os.Args) < 2 {
			fmt.Printf("Usage: echorpc [c fam addr call] nbytes iter | [s port]")
		}

	*/

	logfile,err := os.Open(Logfile, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		log.Panic("No log file", err)
	}
	log.SetOutput(logfile)
log.Printf("DoPrivateMount: %v\n", DoPrivateMount)
	if *DebugLevel > -1 {
		log.Printf("gproc starts with %v and *DebugLevel is %d\n", os.Args, *DebugLevel)
	}
	switch flag.Arg(0) {
	/* traditional bproc master, commands over unix domain socket */
	case "d":
		SetDebugLevelRPC(flag.Arg(1), flag.Arg(2), flag.Arg(3))
	case "m":
		if len(flag.Args()) < 2 {
			fmt.Printf("Usage: %s m <path>\n", os.Args[0])
			os.Exit(1)
		}
		master(flag.Arg(1))
	case "s":
		/* traditional slave; connect to master, await instructions */
		if len(flag.Args()) < 3 {
			fmt.Printf("Usage: %s s <family> <address>\n", os.Args[0])
			os.Exit(1)
		}
		slave(flag.Arg(1), flag.Arg(2))
	case "e":
		var uniquefiles int = 0
		cmds := make([]Acmd, 0)
		if len(flag.Args()) < 6 {
			fmt.Printf("Usage: %s e  <server address> <fam> <address> <nodes> <command>\n",os.Args[0])
			os.Exit(1)
		}
		var flist vector.Vector
		allfiles := make(map[string]bool, 1024)
		workers, l := iowaiter(flag.Arg(2), flag.Arg(3), len(flag.Arg(4)))
		nodelist := NodeList(flag.Arg(4))
		if len(takeout) > 0 {
			takeaway := strings.Split(takeout, ",", -1)
			for _, s := range takeaway {
				packfile(s, "", &flist, true)
			}
		}
		e, _ := ldd.Ldd(flag.Arg(5), root, libs)
		if !*localbin {
			for _, s := range e {
				packfile(s, root, &flist, false)
			}
		}
		if len(flist) > 0 {
			cmds = make([]Acmd, len(flist))
			listlen := flist.Len()
			uniquefiles = 0
			for i := 0; i < listlen; i++ {
				x := flist.Pop().(*Acmd)
				if _, ok := allfiles[x.name]; !ok {
					cmds[uniquefiles] = *x
					uniquefiles++
					allfiles[x.name] = true
				}
			}
		}

		args := flag.Args()[5:]
		mexecclient("unix", flag.Arg(1), nodelist, []string{}, cmds[0:uniquefiles], args, l, workers)
	case "R":
		run()
	default:
		for _, s := range flag.Args() {
			fmt.Print(s, " ")
		}
		flag.Usage()
	}

}


