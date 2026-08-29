package contract

type Kind string

const (
	KindString  Kind = "string"
	KindInteger Kind = "integer"
	KindNumber  Kind = "number"
	KindBoolean Kind = "boolean"
	KindArray   Kind = "array"
	KindModel   Kind = "model"
	KindEnum    Kind = "enum"
)

type Document struct {
	Title      string
	Version    string
	ServerURL  string
	Models     []Model
	Enums      []Enum
	Operations []Operation
}

type Model struct {
	Name   string
	Fields []Field
}

type Field struct {
	Name     string
	Required bool
	Type     Type
}

type Enum struct {
	Name   string
	Values []string
}

type Type struct {
	Kind     Kind
	Name     string
	Format   string
	Nullable bool
	Items    *Type
}

type Operation struct {
	ID         string
	Method     string
	Path       string
	Scopes     []string
	Parameters []Parameter
	Response   Response
}

type Parameter struct {
	Name     string
	Location ParameterLocation
	Required bool
	Default  any
	Type     Type
}

type ParameterLocation string

const (
	ParameterPath  ParameterLocation = "path"
	ParameterQuery ParameterLocation = "query"
)

type Response struct {
	Status int
	Type   *Type
}
