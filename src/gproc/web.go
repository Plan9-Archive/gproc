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
	"io"
	"net/http"
	"net/url"
	"html/template"
	"os"
	"os/exec"
)

var fmap = template.FuncMap{
	"html":     template.HTMLEscaper,
	"url+html": UrlHtmlFormatter,
}

var (
	templStatus *template.Template
	templExtendedSlaveInformation *template.Template
	templHeader *template.Template
	templFooter *template.Template
)


func web() {
	templStatus, _ = template.New("templStatus").Funcs(fmap).ParseFiles("html/status.template")
	templExtendedSlaveInformation, _ = template.New("templExtendedSlaveInformation").Funcs(fmap).ParseFiles("html/extended-slave-information.template")
	templHeader, _ = template.New("templHeader").Funcs(fmap).ParseFiles("html/header.template")
	templFooter, _ = template.New("templFooter").Funcs(fmap).ParseFiles("html/footer.template")
	if templStatus == nil || templExtendedSlaveInformation == nil || templHeader == nil || templFooter == nil {
		log_info("Not starting web server")
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
		log_info("ListenAndServe:", err)
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
		log_info("status failed: ", b, " with err: ", err)
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
