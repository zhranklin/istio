#!/usr/bin/env bash
export HUB=hub.c.163.com/qingzhou/istio
branch=$(git symbolic-ref --short -q HEAD)
commit=$(git rev-parse --short HEAD)
tag=$(git show-ref --tags| grep $commit | awk -F"[/]" '{print $3}')
if [ -z $tag ]
then
   export TAG=$branch-$commit
else
   export TAG=$branch-$tag
fi

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make $1 &&
echo ${HUB}/$1:${TAG} &&
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make push.docker.$1