#!/bin/bash

a='\033[0;31m' # red
t='\033[0;32m' # green
c='\033[0;33m' # orange
g='\033[0;34m' # blue
w='\033[0m' # No Color

l='\033[0m/'
r='\033[0m\\'

bases=(${a}'|' ${t}'|' ${c}'|' ${g}'|')
entries=($(shuf -r -i 0-3 -n 29))
fp=()
tp=()

for i in ${entries[@]}
do
    fp+=(${bases[$i]}) 
    
    if [[ $i -eq 0 ]]
    then 
        tp+=(${bases[1]}) 
    elif [[ $i -eq 1 ]]
    then 
        tp+=(${bases[0]}) 
    elif [[ $i -eq 2 ]]
    then 
        tp+=(${bases[3]}) 
    else
        tp+=(${bases[2]}) 
    fi
done

# need to figure out non-manual way. Maybe find out how to fit width of terminal?
# at-g34---567-890---123-456---789-012---345-tga 
# t---123-456---789-012---345-678---901-234---ct

echo -e "
 -. .-.   .-. .-.   .-. .-.   .-. .-.   .-. .-.
 ${a}|${t}|${l}${g}|${fp[3]}${fp[4]}${r} ${l}${fp[5]}${fp[6]}${fp[7]}${r}${fp[8]}${fp[9]}${fp[10]}${r} ${l}${fp[11]}${fp[12]}${fp[13]}${r}${fp[14]}${fp[15]}${fp[16]}${r} ${l}${fp[17]}${fp[18]}${fp[19]}${r}${fp[20]}${fp[21]}${fp[22]}${r} ${l}${fp[23]}${fp[24]}${fp[25]}${r}${t}|${g}|${a}|
 ${t}|${l} ${r}${tp[3]}${tp[4]}${tp[1]}${r}${tp[2]}${tp[5]}${tp[6]}${l} ${r}${tp[9]}${tp[10]}${tp[7]}${r}${tp[8]}${tp[11]}${tp[12]}${l} ${r}${tp[15]}${tp[16]}${tp[13]}${r}${tp[14]}${tp[17]}${tp[18]}${l} ${r}${tp[21]}${tp[22]}${tp[19]}${r}${tp[20]}${tp[23]}${tp[24]}${l} ${r}${c}|${t}|
 ${w}~   \`-~ \`-\`   \`-~ \`-\`   \`-~ \`-~   \`-~ \`-\`   \`-"
