package main

import (
	"os"
)


type jaguar struct {
	parentAddr string
	addr string
}

func init() {
	addLocale("jaguar", new(jaguar))
}

func (s *jaguar) initHostTable() {
}

func (s *jaguar) Init(role string) {
		switch role {
		case "master":
			cmdPort = "6666"
			/* we hardwire this because the LocalAddr of a 
			 * connected socket has an address of 0.0.0.0 !!
			 */
			s.addr = "192.168.30.69:" + cmdPort
			s.parentAddr = ""
		case "slave":
			cmdPort = "6666"
			s.parentAddr = "192.168.30.69:" + cmdPort
			s.addr = "0.0.0.0:" + cmdPort
		case "client", "run":
		}
}

func (s *jaguar) ParentAddr() string {
	return s.parentAddr
}

func (s *jaguar) Addr() string {
	return s.addr
}

func (s *jaguar) RegisterServer(l Listener) (err os.Error) {
	return
}
