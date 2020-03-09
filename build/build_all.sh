go build -o /output/gtunnel-server-linux server/server.go
go build -o /output/gtunnel-client-linux client/client.go
set GOOS=windows
set GOARCH=386
go build -o /output/gtunnel-server-win.exe server/server.go
go build -o /output/gtunnel-client-win.exe client/client.go
