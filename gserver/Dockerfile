FROM golang:1.14

WORKDIR /go/src/gTunnel

RUN mkdir /output

RUN apt update && apt install -y \
	unzip \
	openssl \
    gcc-mingw-w64-i686 \
    gcc-mingw-w64-x86-64

COPY grpc/ grpc/.
COPY build/build_protoc.sh .

RUN go get -u google.golang.org/grpc
RUN wget https://github.com/protocolbuffers/protobuf/releases/download/v3.11.4/protoc-3.11.4-linux-x86_64.zip && \
	unzip protoc-3.11.4-linux-x86_64.zip -d /protoc
ENV PATH=$PATH:/protoc/bin
RUN go get -u github.com/golang/protobuf/protoc-gen-go
ENV PATH=$PATH:$GOPATH/bin

RUN ./build_protoc.sh

COPY go.mod .
COPY build/build_all.sh .
COPY gserver/ gserver/.
COPY gclient/ gclient/.
COPY common/ common/.

RUN go get -d -v ./...
RUN go install -v ./...

RUN mkdir tls && openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -out tls/cert -subj "/C=/ST=/L=/O=/CN=" -keyout tls/key

CMD ["go", "run", "gserver/gServer.go"]
