/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the Lesser Gnu Programming License, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

package main

import (
	"log"
	"os"
	"strings"
	"bitbucket.org/npe/ldd"
	"io"
	"path"
	"fmt"
)

func startExecution(masterAddr, fam, ioProxyPort, slaveNodes string, cmd []string) {
	log.SetPrefix("mexec " + *prefix + ": ")
	/* make sure there is someone to talk to, and get the vital data */
	client, err := Dial("unix", "", masterAddr)
	if err != nil {
		log.Fatal("startExecution: dialing: ", fam, " ", masterAddr, " ", err)
	}
	r := NewRpcClientServer(client)

	/* master sends us vital data */
	var vitalData vitalData
	r.Recv("vitalData", &vitalData)
	pv := newPackVisitor()
	cwd, _ := os.Getwd()
	/* make sure our cwd ends up in the list of things to take along ...  but only take the dir*/
	path.Walk(cwd + "/.", pv, nil);
	if len(*filesToTakeAlong) > 0 {
		files := strings.Split(*filesToTakeAlong, ",", -1)
		for _, f := range files {
			rootedpath := f
			if f[0] != '/' {
				rootedpath = cwd + "/"  + f
			} 
			path.Walk(rootedpath, pv, nil)
		}
	}
	rawFiles, _ := ldd.Ldd(cmd[0], *root, *libs)
	Dprint(4, "LDD say rawFiles ", rawFiles, "cmds ", cmd, "root ", *root, " libs ", *libs)

	/* now filter out the files we will not need */
	finishedFiles := []string{}
	for _, s := range(rawFiles) {
		if len(vitalData.Exceptlist) > 0 && vitalData.Exceptlist[s] {
			continue
		}
		finishedFiles = append(finishedFiles, s)
	}
	if !*localbin {
		for _, s := range finishedFiles {
			/* WHAT  A HACK -- ldd is really broken. HMM, did not used to be!*/
			if s == "" {
				continue
			}
			Dprint(4, "startExecution: not local walking '", s, "' full path is '", *root+s, "'")
			path.Walk(*root+s, pv, nil)
			Dprint(4, "finishedFiles is ", finishedFiles)
		}
	}
	/* build the library list given that we may have a different root */
	libList := strings.Split(*libs, ":", -1)
	rootedLibList := []string{}
	for _, s := range libList {
		Dprint(6, "startExecution: add lib ", s)
		rootedLibList = append(rootedLibList, fmt.Sprintf("%s/%s", *root, s))
	}
	/* this test could be earlier. We leave it all the way down here so we can 
	 * easily test the library code. Later, it can move
	 * earlier in the code. 
	 */
	if !vitalData.HostReady {
		fmt.Print("Can not start jobs: ", vitalData.Error, "\n")
		return
	}
	Dprint(4, "startExecution: libList ", libList)
	ioProxyListenAddr := vitalData.HostAddr + ":" + ioProxyPort
	workerChan, l, err := ioProxy(fam, ioProxyListenAddr)
	if err != nil {
		log.Fatal("startExecution: ioproxy: ", err)
	}

	req := StartReq{
		Command:         "e",
		Lfam:            l.Addr().Network(),
		Lserver:         l.Addr().String(),
		LocalBin:        *localbin,
		Args:            cmd,
		BytesToTransfer: pv.bytesToTransfer,
		LibList:         libList,
		Path:            *root,
		Nodes:           slaveNodes,
		Cmds:            pv.cmds,
		PeerGroupSize:   *peerGroupSize,
		Cwd:		cwd,
	}

	r.Send("startExecution", req)
	resp := &Resp{}
	r.Recv("startExecution", resp)
	numWorkers := resp.NumNodes
	Dprintln(3, "startExecution: waiting for ", numWorkers)
	for numWorkers > 0 {
		<-workerChan
		numWorkers--
	}
	Dprintln(3, "startExecution: finished")
}

func writeOutFiles(r *RpcClientServer, root string, cmds []*cmdToExec) {
	for _, c := range cmds {
		Dprint(2, "writeOutFiles: next cmd")
		if !c.Fi.IsRegular() {
			continue
		}
		fullpath := root + c.FullPath
		f, err := os.Open(fullpath, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("Open %v failed: %v\n", fullpath, err)
		}
		Dprint(2, "writeOutFiles: copying ", c.Fi.Size, " from ", f)
		// us -> master -> slaves
		n, err := io.Copyn(r.ReadWriter(), f, c.Fi.Size)
		Dprint(2, "writeOutFiles: wrote ", n)
		f.Close()
		if err != nil {
			log.Fatal("writeOutFiles: copyn: ", err)
		}
	}
	Dprint(2, "writeOutFiles: finished")
}


func isNum(c byte) bool {
	return '0' <= c && c <= '9'
}

var (
	BadRangeErr = os.NewError("bad range format")
)

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
	//	_, file := path.Split(filePath)
	c := &cmdToExec{
		//		name: file,
		Name:     filePath,
		FullPath: filePath,
		Local:    0,
		Fi:       f,
	}
	Dprint(4, "VisitDir: appending ", filePath, " ", []byte(filePath), " ", p.alreadyVisited)
	p.cmds = append(p.cmds, c)
	p.alreadyVisited[filePath] = true
	/* to make it possible to drag directories along, without dragging files along, we adopt that convention that 
	 * if the user ends a dir with /., then we won't recurse
	 */
	if strings.HasSuffix(filePath, "/.") {
		return false
	}
	return true
}

func isNull(r int) bool {
	return r == 0
}

func (p *packVisitor) VisitFile(filePath string, f *os.FileInfo) {
	// shouldn't need to do this, need to fix ldd
	filePath = strings.TrimSpace(filePath)
	filePath = strings.TrimRightFunc(filePath, isNull)
	if p.alreadyVisited[filePath] {
		return
	}
	c := &cmdToExec{
		//		name: file,
		Name:     filePath,
		FullPath: filePath,
		Local:    0,
		Fi:       f,
	}
	Dprint(4, "VisitFile: appending ", f.Name, " ", f.Size, " ", []byte(filePath), " ", p.alreadyVisited)

	p.cmds = append(p.cmds, c)

	switch {
	case f.IsRegular():
		p.bytesToTransfer += f.Size
	case f.IsSymlink():
		c.FullPath = resolveLink(filePath)
		path.Walk(c.FullPath, p, nil)
	}
	p.alreadyVisited[filePath] = true
}

func resolveLink(filePath string) string {
	// BUG: what about relative paths in the link?
	linkPath, err := os.Readlink(filePath)
	linkDir, linkFile := path.Split(linkPath)
	switch {
	case linkDir == "":
		linkDir, _ = path.Split(filePath)
	case linkDir[0] != '/':
		dir, _ := path.Split(filePath)
		linkDir = path.Join(dir, linkDir)
	}
	Dprint(4, "VisitFile: read link ", filePath, "->", linkDir+linkFile)
	if err != nil {
		log.Fatal("VisitFile: readlink: ", err)
	}
	return path.Join(linkDir, linkFile)
}
