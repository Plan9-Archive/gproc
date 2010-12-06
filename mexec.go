package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"bitbucket.org/npe/ldd"
	"strconv"
	"io"
	"path"
)

func startExecution(masterAddr, fam, server, nodes string, cmd []string) {
	log.SetPrefix("mexec " + *prefix + ": ")
	workerChan, l, err := ioProxy(fam, server, len(nodes))
	if err != nil {
		log.Exit("startExecution: ioproxy: ", err)
	}

	nodelist, err := NodeList(nodes)
	if err != nil {
		log.Exit("startExecution: bad nodelist: ", err)
	}

	pv := newPackVisitor()
	if len(*filesToTakeAlong) > 0 {
		files := strings.Split(*filesToTakeAlong, ",", -1)
		for _, f := range files {
			path.Walk(f, pv, nil)
		}
	}
	e, _ := ldd.Ldd(cmd[0], *root, *libs)
	if !*localbin {
		for _, s := range e {
			Dprint(4, "startExecution: not local walking ", *root+s)
			path.Walk(*root+s, pv, nil)
		}
	}
	req := StartReq{
		Lfam:            l.Addr().Network(),
		Lserver:         l.Addr().String(),
		LocalBin:        *localbin,
		Args:            cmd,
		bytesToTransfer: pv.bytesToTransfer,
		Env:             []string{"LD_LIBRARY_PATH=/tmp/xproc/lib:/tmp/xproc/lib64"},
		Nodes:           nodelist,
		cmds:            pv.cmds,
	}
	client, err := Dial("unix", "", masterAddr)
	if err != nil {
		log.Exit("startExecution: dialing: ", fam, " ", server, " ", err)
	}
	r := NewRpcClientServer(client)
	r.Send("startExecution", req)
	writeOutFiles(r, pv.cmds)
	r.Recv("startExecution", &Resp{})
	peers := []string{} // TODO
	numWorkers := len(nodes) + len(peers)
	for numWorkers > 0 {
		<-workerChan
		numWorkers--
	}
}

func writeOutFiles(r *RpcClientServer, cmds []*cmdToExec) {
	for _, c := range cmds {
		Dprint(2, "writeOutFiles: next cmd")
		if !c.fi.IsRegular() {
			continue
		}
		f, err := os.Open(c.fullpathname, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("Open %v failed: %v\n", c.fullpathname, err)
		}
		Dprint(2, "writeOutFiles: copying ", c.fi.Size, " from ", f)
		// send to master to send to others
		n, err := io.Copyn(r.ReadWriter(), f, c.fi.Size)
	//	n, err := io.Copy(r.ReadWriter(), f)
		Dprint(2, "writeOutFiles: wrote ", n)
		f.Close()
		if err != nil {
			log.Exit("writeOutFiles: copyn: ", err)
		}
	}
	Dprint(2, "writeOutFiles: finished")
}

func ioProxy(fam, server string, numWorkers int) (workerChan chan int, l Listener, err os.Error) {
	l, err = Listen(fam, server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Listen: %v\n", err)
		return
	}

	workerChan = make(chan int, numWorkers)
	Workers := make([]*Worker, numWorkers)
	go func() {
		for i, _ := range Workers {
			conn, err := l.Accept()
			Dprint(2, "ioProxy: connected by ", conn.RemoteAddr())
			w := &Worker{Alive: true, Conn: conn, Status: workerChan}
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
			j := i + 1
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
			} else if i < len(l) {
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

type packVisitor struct {
	cmds            []*cmdToExec
	alreadyVisited  map[string]bool
	bytesToTransfer int64
}

func newPackVisitor() (p *packVisitor) {
	return &packVisitor{alreadyVisited: make(map[string]bool)}
}

func (p *packVisitor) VisitDir(filePath string, f *os.FileInfo) bool {
	filePath = strings.TrimSpace(filePath)
	filePath = strings.TrimRightFunc(filePath, isNull)
	
	if p.alreadyVisited[filePath] {
		return false
	}
	// _, file := path.Split(filePath)
	c := &cmdToExec{
		name: filePath,
		// name:         file,
		fullpathname: filePath,
		local:        0,
		fi:           f,
	}
	Dprint(4, "VisitDir: appending ", filePath, " ", []byte(filePath), " ", p.alreadyVisited)
	p.cmds = append(p.cmds, c)
	p.bytesToTransfer += f.Size
	p.alreadyVisited[filePath] = true
	p.alreadyVisited[filePath] = true
	
	return true
}

func isNull(r int) bool {
	return r == 0
}

func (p *packVisitor) VisitFile(filePath string, f *os.FileInfo) {
	// shouldn't need to do this, need to fix ldd
	filePath = strings.TrimSpace(filePath)
	filePath = strings.TrimRightFunc(filePath, isNull)
	if  p.alreadyVisited[filePath] {
		return
	}
	// _, file := path.Split(filePath)
	c := &cmdToExec{
		name: filePath,
		// name:         file,
		fullpathname: filePath,
		local:        0,
		fi:           f,
	}
	Dprint(4, "VisitFile: appending ", filePath, " ", f.Size, " ", []byte(filePath), " ", p.alreadyVisited)

	p.cmds = append(p.cmds, c)
	if f.IsRegular() {
		p.bytesToTransfer += f.Size
	}
	p.alreadyVisited[filePath] = true	
}
