/* these are variables which it makes no sense to have as options. 
 * at the same time, a json-style file makes no sense either; we have to carry it along and it 
 * does not express computation well. They are determined from your 
 * location and in many cases they will end up being computed. 
 */

package main

import (
	"io/ioutil"
	"log"
)
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
	}
}
