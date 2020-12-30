protoc -I grpc/client grpc/client/client.proto --go_out=plugins=grpc:grpc/client
protoc -I grpc/admin grpc/admin/admin.proto --go_out=plugins=grpc:grpc/admin
