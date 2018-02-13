# GRPC

````bash
# TODO: install protoc, it seems I installed it using package manager
# this will install the binary, which is required by protoc
go get -u github.com/gogo/protobuf/protoc-gen-gogo
````

> gRPC Server Reflection provides information about publicly-accessible gRPC services on a server, and assists clients at runtime to construct RPC requests and responses without precompiled service information. It is used by gRPC CLI, which can be used to introspect server protos and send/receive test RPCs.