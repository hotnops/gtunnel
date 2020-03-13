go build -o /output/gtunnel-server-linux gServer/gServer.go
go build -o /output/gtunnel-client-linux gClient/gClient.go
set GOOS=windows
set GOARCH=386
go build -o /output/gtunnel-server-win.exe gServer/gServer.go
go build -o /output/gtunnel-client-win.exe gClient/gClient.go
