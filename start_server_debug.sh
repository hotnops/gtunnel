docker run -it --rm -p 5555:5555 -p 4444:4444 -p 2345:2345 -v ${PWD}:/go/src/gTunnel/ --name gtun-server-debug --privileged gtunnel-server-debug
