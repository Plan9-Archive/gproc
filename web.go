/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the GNU Lesser General Public License, version 2, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

package main

import (
	"http"
	"io"
	"log"
	"old/template"
	"exec"
	"os"
	"url"
)

var fmap = template.FormatterMap{
	"html":     template.HTMLFormatter,
	"url+html": UrlHtmlFormatter,
}

var templStatus, _ = template.ParseFile("html/status.template", fmap)
var templExtendedSlaveInformation, _ = template.ParseFile("html/extended-slave-information.template", fmap)
var templHeader, _ = template.ParseFile("html/header.template", fmap)
var templFooter, _ = template.ParseFile("html/footer.template", fmap)

func web() {
	if templStatus == nil || templExtendedSlaveInformation == nil || templHeader == nil || templFooter == nil {
		log.Print("Not starting web server")
		return
	}
	// Static pages:
	// Register the dojo files
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, "html/home.html")
	}))

	// Then, dynamic pages:
	// Register the status handles
	http.Handle("/status", http.HandlerFunc(Status))
	http.Handle("/extended-slave-information", http.HandlerFunc(ExtendedSlaveInformation))

	// Finally, start the web server
	err := http.ListenAndServe(":9000", nil)
	if err != nil {
		log.Print("ListenAndServe:", err)
	}
}

func Status(w http.ResponseWriter, httpReq *http.Request) {
	// This will execute an "e" command and return the output
	// NOTE: soon we should fix the nasty ././././././. for length
	// of tree
	// NO: Just add another interface to local to return "depth". 
	// Let's start using the locale more . 
	argv := []string{"-locale=etchosts", "-localbin=true",
		"-merger=true", "-iopp=4445", "e", "././././././.", "/bin/echo",
		"up"}

	cmd := exec.Command(os.Args[0], argv...)
	cmd.Env = os.Environ()
	b, err := cmd.Output()
	if err != nil {
		log.Print("status failed: ", b, " with err: ", err)
	}

	// We need to build the page
	data := map[string]interface{}{
		"title":  "Status",
		"return": string(b[:]),
	}

	templHeader.Execute(w, data)
	templStatus.Execute(w, data)
	templFooter.Execute(w, data)
}

func ExtendedSlaveInformation(w http.ResponseWriter, req *http.Request) {
	// Get the list of servers
	var slavesOut []SlaveInfo
	for _, i := range slaves.Slaves {
		slavesOut = append(slavesOut, *i)
	}
	data := map[string]interface{}{
		"title":     "Extended Slave Information",
		"slavesOut": slavesOut,
	}

	templHeader.Execute(w, data)
	templExtendedSlaveInformation.Execute(w, data)
	templFooter.Execute(w, data)
}

func UrlHtmlFormatter(w io.Writer, fmt string, v ...interface{}) {
	template.HTMLEscape(w, []byte(url.QueryEscape(v[0].(string))))
}
