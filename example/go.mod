module main

go 1.17

require (
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/leon-auradine/golang-jwt-tpm v0.0.0-20221011180900-ce6fea9e05f7
)

require (
	github.com/google/go-tpm v0.3.2 // indirect
	github.com/google/go-tpm-tools v0.3.1 // indirect
	golang.org/x/sys v0.0.0-20210316092937-0b90fd5c4c48 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
)

replace github.com/salrashid123/golang-jwt-tpm => ../
