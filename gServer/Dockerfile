FROM golang:1.14

WORKDIR /go/src/gTunnel

RUN mkdir /output

RUN apt update && apt install -y \
	unzip

COPY gTunnel/ gTunnel/.
COPY build/build_protoc.sh .

RUN go get -u google.golang.org/grpc
RUN wget https://github.com/protocolbuffers/protobuf/releases/download/v3.11.4/protoc-3.11.4-linux-x86_64.zip && \
	unzip protoc-3.11.4-linux-x86_64.zip -d /protoc
ENV PATH=$PATH:/protoc/bin
RUN go get -u github.com/golang/protobuf/protoc-gen-go
ENV PATH=$PATH:$GOPATH/bin

RUN ./build_protoc.sh

COPY build/build_all.sh .
COPY server/ server/.
COPY client/ client/.
COPY common/ common/.

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["go", "run", "server/server.go"]


	

