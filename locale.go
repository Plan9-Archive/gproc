/* these are variables which it makes no sense to have as options. 
 * at the same time, a json-style file makes no sense either; we have to carry it along and it 
 * does not express computation well. They are determined from your 
 * location and in many cases they will end up being computed. 
 */

package main

import (
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
)

func getOurIPs() []string {
	hostName, err := os.Hostname()
	if err != nil {
		log.Exit(err)
	}
	if addrs, ok := hostMap[hostName]; ok {
		return addrs
	}
	_, addrs, err := net.LookupHost(hostName)
	if err != nil {
		log.Exit(err)
	}
	return addrs
}

/*

so how does this get set up?
what ron is actually doing is take something that should be an interface. 


*/


type Locale interface {
	Init(role string)
}

var locales = map[string]Locale{
	"local":     &local{},
	"strongbox": &strongbox{},
}

type local struct{}

func (l local) Init(role string) {
	switch role {
	case "master", "slave":
		cmd, err := ioutil.ReadFile(srvAddr)
		if err != nil {
			log.Exit(err)
		}
		parentCmdSocket = "127.0.0.1:" + string(cmd)
	case "client", "run":
	}
}

type strongbox struct {
}

func (l strongbox) Init(role string) {
		/* set up hostMap */
		hostMap = make(map[string][]string)
		for i := 0; i < 197; i++ {
			n := strconv.Itoa(i)
			host := "cn" + n
			ip := "10.0.0." + n
			hostMap[host] = []string{ip}
		}
		addrs := getOurIPs()
		switch role {
		case "master":
			cmdPort = "6666"
			/* we hardwire this because the LocalAddr of a 
			 * connected socket has an address of 0.0.0.0 !!
			 */
			myCmdSocket = "10.0.0.254:" + cmdPort
			parentCmdSocket = ""
		case "slave":
			cmdPort = "6666"
			/* on strongbox there's only ever one.
			 * pick out the lowest-level octet.
			 */
			b := net.ParseIP(addrs[0]).To4()
			which := b[3]
			switch {
			case which%7 == 0:
				parentCmdSocket = "10.0.0.254:6666"
			default:
				boardMaster := ((which + 6) / 7) * 7
				parentCmdSocket = "10.0.0." + string(boardMaster) + ":6666"
			}
			myCmdSocket = b.String() + cmdPort
		case "client", "run":
		}
}

