# gTunnel
A TCP tunneling suite built with golang and gRPC. gTunnel can manage multiple forward and reverse tunnels that are all carried over a single TCP/HTTP2 connection. I wanted to learn a new language, so I picked go and gRPC. Client executables have been tested on windows and linux.

# Dependencies
Docker

redis

# I Need A Remote SOCKS Proxy ASAFP
If you don't care to read about how gTunnel works and just need a tunnel, like, right now, then you want to start here. You will need to do five things. It should take about 5-10 minutes.

1) Start the gtunnel server
2) Build a client
3) Build or download the gtuncli
4) Register the client with the server
5) Start the client on the remote host
6) Add the tunnel and socks server


## Start the gtunnel server
Pull down the latest gtunnel-server image. Make sure port 443 is open on your host.
<pre>
apt install redis
docker pull hotnops/gtunnel-server:latest
mkdir logs
mkdir tls
# If you have certificates from letsencrypt or something, just make sure to put them in the tls folder that gets mounted
# and named "key" and "cert"
cd tls && openssl req -new -newkey rsa:4096 -x509 -sha256 -days 365 -nodes -out cert -subj "/C=/ST=/L=/O=/CN=" -keyout key && cd ..
docker run --net host -v $PWD/logs:/logs -v $PWD/tls:/tls --name gtun-server hotnops/gtunnel-server
</pre>
There should be a new log file in the logs directory that was mounted. It's a good idea to keep an eye on the logs for the rest of setup.
```
tail -f logs/<gtunnel log file>
```

## Build a client
If you haven't already, download the source from github
<pre>
git clone https://github.com/hotnops/gtunnel.git gtunnel
cd gtunnel
</pre>
Run the build client script, the first time might take a minute since it needs to build the docker image.
```
./build_client.sh -arch x64 -bintype exe -ip <public ip of gserver> -name asafp -outputfile asafp.exe -platform win
```
There should now be an executable named *asafp.exe* in the *build* directory. This is the binary that gets deployed to the remote host.

## Build or download the gtuncli
At the moment, I'm not hosting the gtuncli binary anywhere, so you'll need to build it. This is the tool that you will use to interact with the gtunnel-server. Run:
<pre>
./build_gtuncli.sh
</pre>
This will produce a binary in gtuncli/build called *gtuncli*.

## Register the client with the server
The gserver instance you stood up in step 1 needs to be aware of the client you built in step 2. If you want an explanation why this step is separate, go to the actual instructions, I'm just trying to get you up and running.
```
export GTUNNEL_HOST=<IP OF GSERVER>
export GTUNNEL_PORT=<ADMIN GSERVER PORT>
./gtuncli clientregister -arch x64 -bintype exe -ip <public ip of gserver> -name asafp -platform win -token <token output from client build step>
```
## Start the client on the remote host
It is now time to run the client on the remote host. Once connected, you should see a relevant message in the logs. If the client executable is just an exe, start it however you would start any other exe. If it's a DLL, the exported function to start gserver is "ExportedMain".

## Add the tunnel and socks server
Last step. You now need to tell the client that you want to setup a forward tunnel and a socks server. First, you need the client instance ID. You can get that by listing out all the connected clients
```
./gtuncli clientlist
```
Using the unique id in the output, we can add a tunnel to that instance
```
./gtuncli tunnelcreate -clientid <id from previous step> -destinationip 127.0.0.1 -destinationport 4444 -listenip 127.0.0.1 -listenport 5555
```
This will forward all traffic from localhost port 5555 to the target on localhost 4444. Lastly, start a socks server on the remote host and have it listen on port 4444.
```
./gtuncli socksstart -clientid <id from previous step> -port 4444
```
Obviously, you should change port numbers to fit your environment. You now have a forward tunnel / socks proxy.

# What is this?
gTunnel is an ecosystem for managing network tunnels between a local server and a remote host. Communication is initiated from the host and all network traffic happens over a single TCP connection. This means that multiple tunnels, each with multiple TCP connections, can all ride over a single HTTP/2 TCP connection. This is in contrast to a CobaltStrike beacon socks proxy, which creates a new TCP/HTTP session every time data needs to be transferred. The result is significantly faster speeds, which enables red-team operators to quickly browse client intra-nets, run C2, or whatever else. gTunnel is composed of three parts:

1) The gTunnel server
2) The gTunnel client
3) The gTunnel cli tool

## gTunnel Server
The gTunnel server is responsible for handling all incoming client connections as well as tasking tunnel creation and socks commands to the client. The bulk of logic is implemented in gTunnel. In a default deployment, gTunnel will listen on two ports:

* 443 - Client GRPC service
* 1337 - Admin GRPC service

The Admin GRPC service is exposed for administrators to task gServer to do things like: register clients, create tunnels, and disconnect. In version 1.0, the administrator would interact with the gTunnel server in a REPL command shell. This has been removed to allow for the implementation of automated deployement and a easier-to-use WebUI.

## gTunnel client
This is a standalone binary, either an executable or library, that is built with parameters like gServer IP and friendly name. The client is the executable that gets deployed to the remote host. The client can be built for windows, linux, or mac and can be compiled into: exe, dll, elf, and so file types.

## gTunnel cli tool
This is the command line tool used to task the gTunnel server. This is currently the only interface to the gTunnel system, and a web based UI is on the way. All of the supported commands and usage instructions are in the "How to use" section.



# How to use.
Using gTunnel has changed significantly since version 1.0, so listen up.
## Setting up gTunnel server
There are currently two s

# "You did this wrong" AKA "How to develop for gTunnel"
## Setting up a dev environment
## Debugging gtunnel


# Known Issues

* The client may not call out in an environment where a network proxy doesn't know how to transition to HTTP/2

# Qs that I think will be FA'd
* Where are the unit test?
* 

# TODO
* Write a gtunnel client in python
* Write a gtunnel client in c#
* Write a gtunnel client in C/Win32
* Write a nix compatible gtunnel client