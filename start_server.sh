docker build -f gServer/Dockerfile . -t gtunnel-server
docker run -it --rm -p 443:443 -v $PWD/configured:/go/src/gTunnel/configured --name gtun-server gtunnel-server
