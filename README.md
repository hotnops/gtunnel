# gTunnel
A TCP tunneling suite built with golang and gRPC. gTunnel can manage multiple forward and reverse tunnels that are all carried over a single TCP/HTTP2 connection. I wanted to learn a new language, so I picked go and gRPC. Client executables have been tested on windows and linux.


# How to use.
The start_server.sh script will build a docker image and start it with no exposed ports. If you plan on using forward tunnels, make sure to map those ports or to change the docker network.

./start_server.sh

This will eventually provide you with the following prompt:
<pre>
       ___________ ____ ___ _______    _______   ___________.____     
   ___ \__    ___/|    |   \\      \   \      \  \_   _____/|    |    
  / ___\ |    |   |    |   //   |   \  /   |   \  |    __)_ |    |    
 / /_/  >|    |   |    |  //    |    \/    |    \ |        \|    |___ 
 \___  / |____|   |______/ \____|__  /\____|__  //_______  /|_______ \
/_____/                            \/         \/         \/         \/

>>> 
</pre>

The first thing to do is generate a client to run on the remote system. For a windows client named "win-client"
<pre>
>>> configclient win 172.17.0.1 443 win-client
</pre>
For a linux client named lclient
<pre>
>>> configclient linux 172.17.0.1 443 lclient
</pre>

This will output a configured executable in the "configured" directory, relative to ./start_server.sh
Once you run the executable on the remote system, you will be notified of the client connecting
<pre>

       ___________ ____ ___ _______    _______   ___________.____     
   ___ \__    ___/|    |   \\      \   \      \  \_   _____/|    |    
  / ___\ |    |   |    |   //   |   \  /   |   \  |    __)_ |    |    
 / /_/  >|    |   |    |  //    |    \/    |    \ |        \|    |___ 
 \___  / |____|   |______/ \____|__  /\____|__  //_______  /|_______ \
/_____/                            \/         \/         \/         \/


>>> configclient linux 127.0.0.1 443 test
>>> 2020/03/20 22:01:47 Endpoint connected: id: test
>>> 
</pre>
To use the newly connected client, type use and the name of the client. Tab completion is supported.
<pre>
>>> use test
(test) >>>  
</pre>
The prompt will change to indicate with which endpoint you're currently working. From here, you can add or remove tunnels. The format is
<pre>
addtunnel (local | remote) listenPort destinationIP destinationPort <name>
</pre>
For example, to open a local tunnel on port 4444 to the ip 10.10.1.5 in the remote network on port 445 and name it "smbtun", the command would be as follows:
<pre>
addtunnel local 4444 10.10.1.5 445 smbtun
</pre>
Similarly, to open a port on the remote system on port 666 and forward all traffic to 192.168.1.10 on port 443 in the local network, the command would be as follows:
<pre>
addtunnel remote 666 192.168.1.10 443
</pre>
Note that the name is optional, and if not provide, will be given random characters as a name. To list out all active tunnels, use the "listtunnels" command.
<pre>
(test) >>> listtunnels
Tunnel ID: smbtun
Tunnel ID: dVck5Zba
</pre>
To delete a tunnel, use the "deltunnel" command:
<pre>
(test) >>> deltunnel smbtun
Deleting tunnel : smbtun
</pre>

To go back and work with another remote system, use the back command:
<pre>
(test) >>> back
>>>  
</pre>
Notice how the prompt has changed to indicate it is no longer working with a particular client. To disconnect a client from the server, you can either issue the "disconnect" command while using the client, or provide the endpoint id in the main menu.
<pre>
(test) >>> disconnect
2020/03/20 22:14:52 Disconnecting test
(test) >>> 2020/03/20 22:14:52 Endpoint disconnected: test
>>> 
</pre>
Or
<pre>
>>> disconnect test
2020/03/20 22:16:00 Disconnecting test
>>> 2020/03/20 22:16:00 Endpoint disconnected: test
>>> 
</pre>
To exit out of the server, run the exit command:
<pre>
>>> exit
</pre>
Note that this will remove the docker container, but any tls generated certificates and configured executables will be in the tls/ and configured/ directories.

# TODO

[x] Reverse tunnel support

[x] Multiple tunnel support

[] Better print out support for tunnels. It should show how many connections are established and ports, etc.

[] Add REST API and implement web UI

[] Dynamic socks proxy support.

[] Authentication between client and server

[] Server configuration file on input with pre-configured tunnels

# Known Issues

* Intenet Explorer is causing the client to lock up on reverse tunnels
* The startup server script should reuse the built image, not create a new one every time.