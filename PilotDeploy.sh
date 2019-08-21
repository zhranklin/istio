#!/bin/bash
export HUB=hub.c.163.com/qingzhou
export TAG=1.1.11-yx-$1
make pilot
make docker.pilot
echo ${HUB}/pilot:${TAG}
docker push ${HUB}/pilot:${TAG}