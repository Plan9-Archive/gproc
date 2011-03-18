/*
 * gproc, a Go reimplementation of the LANL version of bproc and the LANL XCPU software. 
 * 
 * This software is released under the GNU Lesser General Public License, version 2, incorporated herein by reference. 
 *
 * Copyright (2010) Sandia Corporation. Under the terms of Contract DE-AC04-94AL85000 with Sandia Corporation, 
 * the U.S. Government retains certain rights in this software.
 */

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

this really needs to be a package if only to get the naming right.

*/


type Locale interface {
	Init(role string)
	ParentAddr() string
	Addr() string
	Ip() string
	RegisterServer(l Listener) (err os.Error)
	SlaveIdFromVitalData(vd *vitalData) string
}

type Configer interface {
	Locale
	ConfigFrom(path string) os.Error
}

var locales map[string]Locale

var once sync.Once

type LocaleHandler struct {
	locales map[string]Locale
}
/*

precedence:
	is it in the registered locales? if so, use that
	if not, can we open it?
		if so it's json, use that

*/

var (
	BadLocaleErr = os.NewError("invalid locale")
)

func newLocale(name string) (loc Locale, err os.Error) {
	Dprint(8, locales)
	var inLocales bool
	if loc, inLocales = locales[name]; inLocales {
		Dprint(8, "found ", name)
		return
	}
	if _, err = os.Lstat(name); err != nil {
		// so what's the point of this? To make sure we can return a ueeless error?
		//goto BadLocale
		return
	}
	for _, l := range locales {
		cfg, ok := l.(Configer)
		if !ok {
			continue
		}
		err := cfg.ConfigFrom(name)
		loc = l
		if err == nil {
			return
		}
	}
	//BadLocale:
	// I am leaving this in with a petulant comment. If we're just going to do errno, 
	// why pretend otherwise? 
	//err = BadLocaleErr
	// "invalid locale". Wow. How about -104843? It's just as meaningful. 
	//oh, right, because errno sucks, and implementing errno but with strings sucks too
	// Sorry, EINVAL just makes me see blood and when Plan 9 and Go do it I get even more upset. 
	err = os.NewError("Invalid Local: " + name)
	return
}

/*

	NewLocale
*/

func addLocale(name string, loc Locale) {
	once.Do(func() {
		locales = make(map[string]Locale)
	})
	locales[name] = loc
}
