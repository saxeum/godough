module github.com/saxeum/godough

go 1.27.4

require (
	github.com/coreos/go-oidc/v3 v3.21.0
	github.com/jackc/pgx/v5 v5.13.0
	github.com/oapi-codegen/runtime v1.9.1
	golang.org/x/oauth2 v0.38.0
	gopkg.in/yaml.v3 v3.0.1 // test-only: the spec is read by TestSpecConstraintsMatchTheService
)

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.3 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/text v0.44.0 // indirect
)
