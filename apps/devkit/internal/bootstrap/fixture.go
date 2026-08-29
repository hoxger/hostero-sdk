package bootstrap

import _ "embed"

const OpenAPIPath = "./openapi/hostero.openapi.json"

//go:embed hostero.openapi.json
var openAPI []byte

func OpenAPI() []byte {
	return append([]byte(nil), openAPI...)
}
