FROM golang:1.13

WORKDIR /go/src/gtunnel
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["server"]
