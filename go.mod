module github.com/leon-auradine/golang-jwt-tpm

go 1.17

replace github.com/google/go-tpm => github.com/leon-auradine/go-tpm v0.0.0-20221012003451-7bb8ae53bcbc

replace github.com/google/go-tpm-tools => github.com/leon-auradine/go-tpm-tools v0.0.0-20221012181903-8b56602a10b1

require (
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/google/go-tpm v0.0.0-00010101000000-000000000000
	github.com/google/go-tpm-tools v0.0.0-00010101000000-000000000000
)

require (
	golang.org/x/sys v0.0.0-20221010170243-090e33056c14 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
)
