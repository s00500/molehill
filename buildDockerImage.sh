#!/bin/sh
GIT_TAG=`git describe --abbrev=0 --tags`

go generate

GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-w -s" -o molehill .

docker build -t molehill:$GIT_TAG -t molehill:latest -t s00500/molehill:$GIT_TAG -t s00500/molehill:latest .
docker push s00500/molehill:$GIT_TAG
docker push s00500/molehill:latest
