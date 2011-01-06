#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
scp -o StrictHostKeyChecking=no /tmp/gproc_linux_arm  root@10.0.0.$i: &
done
wait

