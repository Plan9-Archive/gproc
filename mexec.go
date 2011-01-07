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
	pv := newPackVisitor()
	if len(*filesToTakeAlong) > 0 {
		files := strings.Split(*filesToTakeAlong, ",", -1)
		for _, f := range files {
			path.Walk(f, pv, nil)
		}
	}
	e, _ := ldd.Ldd(cmd[0], *root, *libs)
	Dprint(4, "LDD say e ", e, "cmds ", cmd, "root ", *root, " libs ", *libs)
	if !*localbin {
		for _, s := range e {
			/* WHAT  A HACK -- ldd is really broken. HMM, did not used to be!*/
			if s == "" {
				continue
			}
			Dprint(4, "startExecution: not local walking '", s, "' full path is '",*root+s, "'")
			path.Walk(*root+s, pv, nil)
			Dprint(4, "e is ", e)
		}
	}
	/* build the library list given that we may have a different root */
	libList := strings.Split(*libs, ":", -1)
	rootedLibList := []string{}
	for _, s := range libList {
		Dprint(6, "startExecution: add lib ", s)
		rootedLibList = append(rootedLibList, fmt.Sprintf("%s/%s", *root, s))
	}
	Dprint(4, "startExecution: libList ", libList)
	client, err := Dial("unix", "", masterAddr)
	if err != nil {
		log.Exit("startExecution: dialing: ", fam, " ", masterAddr, " ", err)
	}
	r := NewRpcClientServer(client)

	/* master sends us vital data */
	var vitalData vitalData
	r.Recv("vitalData", &vitalData)
	if ! vitalData.HostReady {
		fmt.Print("Can not start jobs: ", vitalData.Error, "\n")
		return
	}
	ioProxyListenAddr := vitalData.HostAddr + ":" + ioProxyPort
	workerChan, l, err := ioProxy(fam, ioProxyListenAddr)
	if err != nil {
		log.Exit("startExecution: ioproxy: ", err)
	}

	req := StartReq{
		Command: "e",
		Lfam:            l.Addr().Network(),
		Lserver:         l.Addr().String(),
		LocalBin:        *localbin,
		Args:            cmd,
		bytesToTransfer: pv.bytesToTransfer,
		LibList:             libList,
		Path:			*root,
		Nodes:           slaveNodes,
		cmds:            pv.cmds,
		peerGroupSize:   *peerGroupSize,
	}

	r.Send("startExecution", req)
	resp := &Resp{}
	r.Recv("startExecution", resp)
	numWorkers := resp.numNodes
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
		if !c.fi.IsRegular() {
			continue
		}
		fullpath := root + c.fullPath
		f, err := os.Open(fullpath, os.O_RDONLY, 0)
		if err != nil {
			log.Printf("Open %v failed: %v\n", fullpath, err)
		}
		Dprint(2, "writeOutFiles: copying ", c.fi.Size, " from ", f)
		// us -> master -> slaves
		n, err := io.Copyn(r.ReadWriter(), f, c.fi.Size)
		Dprint(2, "writeOutFiles: wrote ", n)
		f.Close()
		if err != nil {
			log.Exit("writeOutFiles: copyn: ", err)
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
		name:     filePath,
		fullPath: filePath,
		local:    0,
		fi:       f,
	}
	Dprint(4, "VisitDir: appending ", filePath, " ", []byte(filePath), " ", p.alreadyVisited)
	p.cmds = append(p.cmds, c)
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
	if p.alreadyVisited[filePath] {
		return
	}
	c := &cmdToExec{
		//		name: file,
		name:     filePath,
		fullPath: filePath,
		local:    0,
		fi:       f,
	}
	Dprint(4, "VisitFile: appending ", f.Name, " ", f.Size, " ", []byte(filePath), " ", p.alreadyVisited)

	p.cmds = append(p.cmds, c)
	switch {
	case f.IsRegular():
		p.bytesToTransfer += f.Size
	case f.IsSymlink():
		c.fullPath = resolveLink(filePath)
		path.Walk(c.fullPath, p, nil)
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
		log.Exit("VisitFile: readlink: ", err)
	}
	return path.Join(linkDir, linkFile)
}
