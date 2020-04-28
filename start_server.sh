docker build -f gServer/Dockerfile . -t gtunnel-server
docker run -it --rm --net host -v $PWD/configured:/go/src/gTunnel/configured --name gtun-server gtunnel-server
