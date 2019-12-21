#!/bin/sh

start_port=1030
COUNTER=0
NUM_VMS=$1
while [  $COUNTER -lt $NUM_VMS ]; do

    echo "Trying port: $start_port"

    if [[ -n $(sudo netstat -lntu | grep ":$start_port ") ]];
    # if sudo netstat -lntu | grep ':$start_port '
    then
        echo "[Error] Port in use"
        let "start_port += 1"
    else
        echo "Running on port:$start_port"
        let COUNTER=COUNTER+1

        RESULT=$(echo 00$COUNTER | tail -c 4)
        go run mp2.go $start_port > "node$RESULT.log" &
        let "start_port += 1"
    fi
    echo "$COUNTER ports up"

done    

#     let COUNTER=COUNTER+1 
# done
# for i in {1..20}
# do                  
#    let "port += 1"
#    echo "Running on port:$port"
#    go run mp2.go $port > "node$i.txt" & 
#    echo "$i"
# done