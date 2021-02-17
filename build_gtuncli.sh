docker build --network host -f gserver/Dockerfile --target gtuncli . -t gtuncli-build-image
docker run --net host --name gtuncli-build -v $PWD/build:/go/src/gTunnel/gtuncli/build gtuncli-build-image
docker rm gtuncli-build
