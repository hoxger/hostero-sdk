package contract

type Kind string

const (
	KindString  Kind = "string"
	KindInteger Kind = "integer"
	KindNumber  Kind = "number"
	KindBoolean Kind = "boolean"
	KindArray   Kind = "array"
	KindMap     Kind = "map"
	KindUnion   Kind = "union"
	KindAny     Kind = "any"
	KindModel   Kind = "model"
	KindEnum    Kind = "enum"
	KindAlias   Kind = "alias"
)

type Document struct {
	Title      string
	Version    string
	ServerURL  string
	Models     []Model
	Enums      []Enum
	Aliases    []Alias
	Operations []Operation
}

type Model struct {
	Name        string
	Description string
	Fields      []Field
}

type Field struct {
	Name        string
	Description string
	Required    bool
	Type        Type
}

type Enum struct {
	Name        string
	Description string
	Values      []string
}

type Alias struct {
	Name        string
	Description string
	Type        Type
}

type Type struct {
	Kind     Kind
	Name     string
	Format   string
	Nullable bool
	Items    *Type
	Values   []Type
}

type ParameterLocation string

const (
	ParameterPath   ParameterLocation = "path"
	ParameterQuery  ParameterLocation = "query"
	ParameterHeader ParameterLocation = "header"
	ParameterCookie ParameterLocation = "cookie"
)

type ClientMetadata struct {
	Group  []string
	Method string
}

type Operation struct {
	ID             string
	Method         string
	Path           string
	Summary        string
	Description    string
	Tags           []string
	Permissions    []string
	TargetKinds    []string
	ClientMetadata ClientMetadata
	Parameters     []Parameter
	RequestBody    *RequestBody
	Success        Response
	Errors         []Response
}

type Parameter struct {
	Name        string
	Description string
	Location    ParameterLocation
	Required    bool
	Type        Type
}

type RequestBody struct {
	Required    bool
	ContentType string
	Type        Type
}

type Response struct {
	Status      int
	ContentType string
	Type        *Type
}
