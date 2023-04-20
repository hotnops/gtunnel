protoc -I=client --go_out=. --go-grpc_out=. client/client.proto
protoc -I=admin --go_out=. --go-grpc_out=. admin/admin.proto 
