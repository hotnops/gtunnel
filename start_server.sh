docker start gtunnel-server &> /dev/null

if test $? -eq 0
then
    echo "[*] Server successfully started"
else
    echo "[!] Failed to start gtunnel server"
fi


