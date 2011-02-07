#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
ssh -q root@kn$i  ./gproc_linux_386 -locale=kf s&
done
wait

