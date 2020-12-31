docker build -f gserver/Dockerfile . -t gtunnel-server
docker rm gtun-server
docker run -it --net host -v $PWD/configured:/go/src/gTunnel/configured -v $PWD/logs:/go/src/gTunnel/logs --name gtun-server gtunnel-server
