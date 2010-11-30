package main

import (
	"fmt"
	"log"
	"os"
	"gob"
	"flag"
	"container/vector"
	"net"
	"strings"
	"bitbucket.org/npe/ldd"
)

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

func mexec() {
	var uniquefiles int = 0
	cmds := make([]Acmd, 0)
	var flist vector.Vector
	allfiles := make(map[string]bool, 1024)
	workers, l := iowaiter(flag.Arg(2), flag.Arg(3), len(flag.Arg(4)))
	nodelist := NodeList(flag.Arg(4))
	if len(*takeout) > 0 {
		takeaway := strings.Split(*takeout, ",", -1)
		for _, s := range takeaway {
			packfile(s, "", &flist, true)
		}
	}
	e, _ := ldd.Ldd(flag.Arg(5), *root, *libs)
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

	args := flag.Args()[5:]
	mexecclient("unix", flag.Arg(1), nodelist, []string{}, cmds[0:uniquefiles], args, l, workers)
}
