docker run -it --rm --net host -v $PWD/configured:/go/src/gTunnel/configured -v $PWD/logs:/go/src/gTunnel/logs --name gtun-server-debug gtunnel-server-debug
