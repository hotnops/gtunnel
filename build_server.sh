#!/bin/bash

docker build --network host -f gserver/Dockerfile --target gtunserver . -t gtunnel-server-image

if test $? -eq 0
then
    docker rm gtunnel-server -f || true
else
    echo "[!] Failed to  build gtunnel server image"
    exit 1
fi

docker create --net host -v $PWD/configured:/go/src/gTunnel/configured -v $PWD/logs:/go/src/gTunnel/logs --name gtunnel-server gtunnel-server-image

if test $? -eq 0
then
    echo "[*] Docker container successfully created"
else
    echo "[!] Failed to create gtunnel-server container"
fi
