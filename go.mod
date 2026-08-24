module github.com/crydensync/csax

go 1.25.0

require (
	github.com/crydensync/cryden/v2 v2.0.0
	github.com/lib/pq v1.12.3
)

require (
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
)

replace github.com/crydensync/cryden/v2 => ../cryden
