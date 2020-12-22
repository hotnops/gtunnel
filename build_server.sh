docker build -f gServer/Dockerfile . -t gtunnel-server
docker rm gtun-server
docker run -it --net host -v $PWD/configured:/go/src/gTunnel/configured --name gtun-server gtunnel-server
