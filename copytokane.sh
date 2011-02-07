#!/bin/bash
for i in `seq $1 $2` 
do
echo $i
scp /lib/ld-* root@kn$i:/lib &
scp -o StrictHostKeyChecking=no gproc_linux_386  root@kn$i: &
done
wait

