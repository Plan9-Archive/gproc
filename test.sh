#!/usr/bin/env bash

if ! which qemu-system-arm; then
	echo 'need qemu arm for tests'
fi


if ![[ test -e ubuntu-qemu.raw ]]; then
	if which wget; then
		getter=wget
	elif which curl; then
		getter=curl
	else
		echo 'need a downloader '		
	fi
	echo 'downloading qemu image'
	$getter http://dl.dropbox.com/u/15800618/ubuntu-arm.bz2
	# may blow some people's hds
	bunzip2 ubuntu-arm.bz2 >ubuntu-qemu.raw
fi

