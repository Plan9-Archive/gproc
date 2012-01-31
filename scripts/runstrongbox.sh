#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
ssh -q root@10.0.0.$i  ./gproc_linux_arm -locale=strongbox s&
done
wait

