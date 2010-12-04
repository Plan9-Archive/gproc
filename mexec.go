package main

import (
	"fmt"
	"log"
	"os"
	"container/vector"
	"strings"
	"bitbucket.org/npe/ldd"
	"io"
)

/*

execute on nodes. 

*/

// should be ...
func mexec(masterAddr, fam, server, nodes string, cmd []string) {
	var uniquefiles int = 0
	var cmds []Acmd
	allfiles := make(map[string]bool, 1024)

	log.SetPrefix("mexec " + *prefix + ": ")
	workers, l, err := iowaiter(fam, server, len(nodes))
	if err != nil {
		log.Exit(err)
	}

	nodelist := NodeList(nodes)
	var flist vector.Vector
	if len(*takeout) > 0 {
		takeaway := strings.Split(*takeout, ",", -1)
		for _, s := range takeaway {
			packfile(s, "", &flist, true)
		}
	}
	e, _ := ldd.Ldd(cmd[0], *root, *libs)
	if !*localbin {
		for _, s := range e {
			packfile(s, *root, &flist, false)
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

	mexecclient("unix", masterAddr, nodelist, []string{}, cmds[0:uniquefiles], cmd, l, workers)
}

func iowaiter(fam, server string, nw int) (workers chan int, l Listener, err os.Error) {
	workers = make(chan int, nw)
	Workers := make([]*Worker, nw)
	l, err = Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %v\n", err)
		return
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
	return
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


func mexecclient(fam, server string, nodes, peers []string, cmds []Acmd, args []string, l Listener, workers chan int) os.Error {
	nworkers := len(nodes) + len(peers)
	var err os.Error
	a := StartArg{Lfam: string(l.Addr().Network()), Lserver: string(l.Addr().String()), cmds: nil, LocalBin: *localbin}
	files := make([]*os.File, len(cmds))
	for i := 0; i < len(cmds); i++ {
		Dprintf(2, "mexecclient: cmd %v\n", cmds[i])
		if !cmds[i].fi.IsRegular() {
			continue
		}
		files[i], err = os.Open(cmds[i].fullpathname, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("Open %v failed: %v\n", cmds[i].fullpathname, err)
		}
		defer files[i].Close()
		a.totalfilebytes += cmds[i].fi.Size
	}
	Dprintf(2, "a.totalfilebytes: %v\n", a.totalfilebytes)
	a.Args = args
	a.Env = []string{"LD_LIBRARY_PATH=/tmp/xproc/lib:/tmp/xproc/lib64"}
	a.Nodes = make([]string, len(nodes))
	a.Nodes = nodes
	a.cmds = cmds
	client, err := Dial(fam, "", server)
	if err != nil {
		log.Exit("mexecclient: dialing: ", fam, " ", server, " ", err)
	}
	r := NewRpcClientServer(client)
	r.Send("mexecclient", a)
	for i := 0; i < len(files); i++ {
		if !cmds[i].fi.IsRegular() {
			continue
		}
		Dprint(2, "mexecclient: copying ", cmds[i].fi.Size)
		_, err = io.Copyn(r.ReadWriter(), files[i], cmds[i].fi.Size)
		if err != nil {
			return nil
		}
	}
	r.Recv("mexecclient", &Res{})
	for ; nworkers > 0; nworkers-- {
		<-workers
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
			log.Printf("Bad number: %v\n", ll[0])
		}
		cnt, err = fmt.Sscanf(ll[1], "%d", &end)
		if cnt != 1 || err != nil {
			log.Printf("Bad number: %v\n", ll[1])
		}
		if start > end {
			log.Printf("%d > %d\n", start, end)
		}
		ret = make([]string, end-start+1)
		for i := start; i <= end; i++ {
			ret[i-start] = fmt.Sprint(i)
		}
	case 1:
		ret = ll
	default:
		log.Print("%s: bogus\n", l)
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
		log.Exit("Bad file: ", root+l, err)
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
		Dprintf(2, "packfile: push %v size %d\n", c.name, fi.Size)
		flist.Push(&c)
		curfile = strings.TrimRightFunc(curfile, notslash)
		curfile = strings.TrimRightFunc(curfile, slash)
		/* we don't dodir on our parents. */
		dodir = false
	}
	return nil
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
