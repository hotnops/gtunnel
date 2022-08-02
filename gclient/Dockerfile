FROM golang:1.18

WORKDIR /go/src/gTunnel
ENV PATH=$PATH:/protoc/bin:$GOPATH/bin

# We need unzip to install protoc
RUN apt update && apt install -y \
    unzip \
    gcc-mingw-w64-i686 \
    gcc-mingw-w64-x86-64

# Install protoc and all dependencies
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    wget https://github.com/protocolbuffers/protobuf/releases/download/v3.11.4/protoc-3.11.4-linux-x86_64.zip && \
    unzip protoc-3.11.4-linux-x86_64.zip -d /protoc && \
    go install github.com/golang/protobuf/protoc-gen-go@latest && \
    rm protoc-3.11.4-linux-x86_64.zip

# Copy over all gtunnel files and directories
COPY go.mod .
COPY gclient/ gclient/.
COPY common/ common/.
COPY grpc/ grpc/.

# Build protoc
RUN cd grpc && ./build_protoc.sh && cd ..

# Get all gtunnel dependencies
RUN go get -d -v ./... && go install -v ./...

# Build client builder
RUN mkdir /output
RUN go build -o gclient/gclient_build gclient/builder/gclient_build.go
ENTRYPOINT ["gclient/gclient_build"]
