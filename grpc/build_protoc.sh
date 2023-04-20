protoc -I=client --go_out=client/ --go-grpc_out=client/ client/client.proto
protoc -I=admin --go_out=admin/ --go-grpc_out=admin/ admin/admin.proto 
