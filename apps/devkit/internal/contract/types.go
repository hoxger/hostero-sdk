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
	Title     string
	Version   string
	ServerURL string
	Models    []Model
	Enums     []Enum
	Aliases   []Alias
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

type Alias struct {
	Name string
	Type Type
}

type Type struct {
	Kind     Kind
	Name     string
	Format   string
	Nullable bool
	Items    *Type
	Values   []Type
}
