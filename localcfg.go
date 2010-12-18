package main

import (
	"os"
	"strings"
	"io/ioutil"
	"log"
)


type local struct{
	parentCmdSocket string
	myCmdSocket string
}

func init() {
	addLocale("local", new(local))
}

func (l *local) Init(role string) {
	switch role {
	case "master", "slave":
		cmd, err := ioutil.ReadFile(srvAddr)
		if err != nil {
			log.Exit(err)
		}
		l.parentCmdSocket = "127.0.0.1:" + string(cmd)
	case "client", "run":
	}
}

func (l *local) ParentCmdSocket() string {
	return l.parentCmdSocket
}

func (l *local) CmdSocket() string {
	return l.myCmdSocket
}

func (loc *local) RegisterServer(l Listener) (err os.Error) {
	/* take the port only -- the address shows as 0.0.0.0 */
	addr := strings.Split(l.Addr().String(), ":", 2)
	return ioutil.WriteFile(srvAddr, []byte(addr[1]), 0644)
}
