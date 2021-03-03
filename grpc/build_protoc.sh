protoc -I client client/client.proto --go_out=plugins=grpc:client/ --go_opt=paths=source_relative
protoc -I admin admin/admin.proto --go_out=plugins=grpc:admin/ --go_opt=paths=source_relative
