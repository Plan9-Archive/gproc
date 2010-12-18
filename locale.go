/* these are variables which it makes no sense to have as options. 
 * at the same time, a json-style file makes no sense either; we have to carry it along and it 
 * does not express computation well. They are determined from your 
 * location and in many cases they will end up being computed. 
 */

package main

import (
	"os"
	"sync"
)

/*

locales are a way of setting up arbitrary topologies given a known network. 
They don't use hadoop style configuration files because many times you want to compute your network topology not derive it from a file. 

This allows an abstract interface for gproc to interact with ad hoc trees.

We also provide a json setup for static topologies as well.

To add your own cluster to gproc you need to implement the Locale interface. 

*/


type Locale interface {
	Init(role string)
	ParentCmdSocket() string
	CmdSocket() string
	RegisterServer(l Listener) (err os.Error)
}

func init() {
	
}

var locales map[string]Locale

var once sync.Once

func addLocale(name string, loc Locale) {
	once.Do(func() {
		locales = make(map[string]Locale)
	})
	locales[name] = loc
}

var parentCmdSocket = "0.0.0.0:0"
var myCmdSocket = "0.0.0.0:0"



