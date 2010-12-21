package main

import (
	"os"
	"strings"
	"io/ioutil"
	"log"
)


type local struct{
	parentAddr string
	addr string
}

func init() {
	addLocale("local", &local{"0.0.0.0:0", "0.0.0.0:0"})
}

func (l *local) Init(role string) {
	switch role {
	case "master", "slave":
		cmd, err := ioutil.ReadFile(srvAddr)
		if err != nil {
			log.Exit(err)
		}
		l.parentAddr = "127.0.0.1:" + string(cmd)
	case "client", "run":
	}
}

func (l *local) ParentAddr() string {
	return l.parentAddr
}

func (l *local) Addr() string {
	return l.addr
}

func (loc *local) RegisterServer(l Listener) (err os.Error) {
	/* take the port only -- the address shows as 0.0.0.0 */
	addr := strings.Split(l.Addr().String(), ":", 2)
	return ioutil.WriteFile(srvAddr, []byte(addr[1]), 0644)
}