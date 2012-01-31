#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
scp -o StrictHostKeyChecking=no gproc_linux_arm  root@sb$i: 
done
wait

