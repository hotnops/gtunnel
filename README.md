# gTunnel
A tunneling suite built with golang and gRPC

# How to build
go build server/server.go

go build client/client.go

# How to use.
First startup the server server.exe.

Then connect the client by running client.exe on the remote system.

On the server console, you will get a prompt. You may issue the following commands:
* addtunnel
* listconns


#TODO

[] Reverse tunnel support

[x] Multiple tunnel support

[] Web UI

[] Multiple tunnel links

[] Dynamic socks proxy support
