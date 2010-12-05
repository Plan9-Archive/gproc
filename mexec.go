package main

import (
	"fmt"
	"log"
	"os"
	"container/vector"
	"strings"
	"bitbucket.org/npe/ldd"
	"strconv"
	"io"
)

func startExecution(masterAddr, fam, server, nodes string, cmd []string) {
	var uniquefiles int = 0
	var cmds []*cmdToExec
	allfiles := make(map[string]bool, 1024)

	log.SetPrefix("mexec " + *prefix + ": ")
	workers, l, err := ioProxy(fam, server, len(nodes))
	if err != nil {
		log.Exit(err)
	}

	nodelist, err := NodeList(nodes)
	if err != nil {
		log.Exit("startExecution: bad nodelist: ", err)
	}
	
	var flist vector.Vector
	if len(*filesToTakeAlong) > 0 {
		files := strings.Split(*filesToTakeAlong, ",", -1)
		for _, f := range files {
			packFile(f, "", &flist, true)
		}
	}
	e, _ := ldd.Ldd(cmd[0], *root, *libs)
	if !*localbin {
		for _, s := range e {
			packFile(s, *root, &flist, false)
		}
	}
	if len(flist) > 0 {
		cmds = make([]*cmdToExec, len(flist))
		listlen := flist.Len()
		uniquefiles = 0
		for i := 0; i < listlen; i++ {
			x := flist.Pop().(*cmdToExec)
			if _, ok := allfiles[x.name]; !ok {
				cmds[uniquefiles] = x
				uniquefiles++
				allfiles[x.name] = true
			}
		}
	}
	cmds = cmds[0:uniquefiles]
	req := StartReq{
		Lfam:           l.Addr().Network(),
		Lserver:        l.Addr().String(),
		LocalBin:       *localbin,
		Args:           cmd,
		totalfilebytes: addFiles(cmds),
		Env:            []string{"LD_LIBRARY_PATH=/tmp/xproc/lib:/tmp/xproc/lib64"},
		Nodes:          nodelist,
		cmds:           cmds,
	}
	client, err := Dial("unix", "", masterAddr)
	if err != nil {
		log.Exit("startExecution: dialing: ", fam, " ", server, " ", err)
	}
	r := NewRpcClientServer(client)
	r.Send("startExecution", req)
	writeOutFiles(r, cmds)
	r.Recv("startExecution", &Resp{})
	peers := []string{} // TODO
	numWorkers := len(nodes) + len(peers)
	for numWorkers > 0 {
		<-workers
		numWorkers--
	}
}

func addFiles(cmds []*cmdToExec) (totalfilebytes int64) {
	for _, c := range cmds {
		Dprintf(2, "sendCommandsAndWriteOutFiles: cmd %v\n", c)
		if !c.fi.IsRegular() {
			continue
		}
		var err os.Error
		c.file, err = os.Open(c.fullpathname, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("Open %v failed: %v\n", c.fullpathname, err)
		}
		totalfilebytes += c.fi.Size
	}
	Dprintf(2, "totalfilebytes: %v\n", totalfilebytes)
	return
}

func writeOutFiles(r *RpcClientServer, cmds []*cmdToExec) {
	for _, c := range cmds {
		defer c.file.Close()
		if !c.fi.IsRegular() {
			continue
		}
		Dprint(2, "writeOutFiles: copying ", c.fi.Size, " from ", c.file)
		_, err := io.Copyn(r.ReadWriter(), c.file, c.fi.Size)
		if err != nil {
			log.Exit("writeOutFiles: copyn: ", err)
		}
	}
}

func ioProxy(fam, server string, numWorkers int) (workers chan int, l Listener, err os.Error) {
	l, err = Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %v\n", err)
		return
	}

	workers = make(chan int, numWorkers)
	Workers := make([]*Worker, numWorkers)
	go func() {
		for i, _ := range Workers {
			conn, err := l.Accept()
			Dprint(2, "ioProxy: connected by ", conn.RemoteAddr())
			w := &Worker{Alive: true, Conn: conn, Status: workers}
			Workers[i] = w
			if err != nil {
				Dprint(2, "ioProxy: accept:", err)
				continue
			}
			go func() {
				Dprint(2, "ioProxy: start reading")
				n, err := io.Copy(os.Stdout, w.Conn)
				Dprint(2, "ioProxy: read ", n)
				if err != nil {
					log.Exit("ioProxy: ", err)
				}
				Dprint(2, "ioProxy: end")
				w.Status <- 1
			}()
		}
	}()
	return
}

func isNum(c byte) bool {
	return '0' <= c && c <= '9'
}

var (
	BadRangeErr = os.NewError("bad range format")
)

func NodeList(l string) (rl []string, err os.Error) {
	for i := 0; i < len(l); {
		switch {
		case isNum(l[i]):
			j := i+1
			for j < len(l) && isNum(l[j]) {
				j++
			}
			beg, _ := strconv.Atoi(l[i:j])
			end := beg
			i = j
			if i < len(l) && l[i] == '-' {
				i++
				j = i
				for j < len(l) && isNum(l[j]) {
					j++
				}
				end, _ = strconv.Atoi(l[i:j])
				i = j
			}
			for k := beg; k <= end; k++ {
				rl = append(rl, strconv.Itoa(k))
			}
			if i < len(l) && l[i] == ',' {
				i++
			}else if i < len(l) {				
				goto BadRange
			}
		default:
			goto BadRange
		}
	}
	return
BadRange:
	err = BadRangeErr
	return
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
func packFile(l, root string, flist *vector.Vector, dodir bool) os.Error {
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
		c := cmdToExec{curfile, root + curfile, 0, *fi, nil}
		Dprintf(2, "packFile: push %v size %d\n", c.name, fi.Size)
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
		packFile(l+"/"+s, "", flist, false)
	}
	return nil
}
