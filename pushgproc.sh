#!/usr/bin/env bash

# strip off common prefix
# scp gproc to those nodes
# how would you parse that. using awk? 

# pusgproc 192.168.1.182-218 
# that is the behavior that he wants. 
awk '$0 ~ /[0-9]?[0-9]?[0-9](-[0-9]?[0-9]?[0-9])*\.[0-9]?[0-9]?[0-9](-[0-9]?[0-9]?[0-9])*\.[0-9]?[0-9]?[0-9](-[0-9]?[0-9]?[0-9])*\.[0-9]?[0-9]?[0-9](-[0-9]?[0-9]?[0-9])*\./{
	if(NF != 4)
		next
	split(a,$0, ".")
	o0 = parserange(a[0])
	o1 = parserange(a[1])
	o2 = parserange(a[2])
	o3 = parserange(a[3])
	for(i in o0)
		for(j in o1)
			for(k in o2)
				for(l in o3)
					print i"."j"."k"."l
}

function parserange(o) {
	n = split(a,o, "-")
	if(n == 1)
		return a
	if(n != 2)
		return c
	for(i = a[0]; i < a[1]; i++){
		a[b]
	}
}
'