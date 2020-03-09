docker build -f build/Dockerfile . -t gtunnel-build
docker run -v $PWD/bin:/output gtunnel-build

