docker build -f gServer/Dockerfile . -t gtunnel-server
docker run -it --rm -p 5555:5555 -p 4444:4444 --name gtun-server gtunnel-server
