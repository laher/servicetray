#!/bin/bash

export id=$(docker-compose ps -q $1)
if [ "$id" == "" ]; then
	exit 1;
fi
docker ps -q --no-trunc| grep -q $id
