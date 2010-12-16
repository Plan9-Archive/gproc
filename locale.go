/* these are variables which it makes no sense to have as options. 
 * at the same time, a json-style file makes no sense either; we have to carry it along and it 
 * does not express computation well. They are determined from your 
 * location and in many cases they will end up being computed. 
 */

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
)

func getOurIPs() ([]string) {
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

func localeInit() {
	switch {
	case *locale == "local":
		switch {
		case role == "master":
		case role == "slave":
			cmd, err := ioutil.ReadFile(srvAddr)
			if err != nil {
				log.Exit(err)
			}
			cmdSocket = "127.0.0.1:" + string(cmd)
		case role == "client":
		case role == "run":
		}
	case *locale == "strongbox":
		/* set up hostMap */
		hostMap = make(map[string][]string, 1024)
		for i := 0; i < 197; i++ {
			hostMap[fmt.Sprintf("cn%d", i)] = []string{fmt.Sprintf("10.0.0.%d", i)}
		}
		addrs := getOurIPs()
		switch {
		case role == "master":
			cmdPort = "6666"
		case role == "slave":
			cmdPort = "6666"
			/* on strongbox there's only ever one.
			 * pick out the lowest-level octet.
			 */
			b := net.ParseIP(addrs[0]).To4()
			which := b[3]
			switch {
			case which % 7 == 0:
				cmdSocket = "10.0.0.254:6666"
			default: 
				boardMaster := ((which + 6) / 7) * 7
				cmdSocket = "10.0.0." + string(boardMaster) + ":6666"
			}
			
			cmdSocket = addrs[0] + ":" + "6666"
		case role == "client":
		case role == "run":
		}
	}
}
