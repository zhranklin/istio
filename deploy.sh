#!/bin/bash
export HUB=hub.c.163.com/qingzhou
branch=$(git symbolic-ref --short -q HEAD)
commit=$(git rev-parse --short HEAD)
tag=$(git show-ref --tags| grep $commit | awk -F"[/]" '{print $3}')
if [ -z $tag ]
then
   export TAG=$branch-$commit
else
   export TAG=$branch-$tag
fi

make $1
make docker.$1
echo ${HUB}/$1:${TAG}
docker push ${HUB}/$1:${TAG}

