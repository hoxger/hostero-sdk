package python

type Document struct {
	Modules []Module
}

type GenerationMetadata struct {
	DevKitVersion string
	OpenAPIPath   string
	Release       string
	SHA256        string
}

type ModuleKind string

const (
	ModuleInit   ModuleKind = "init"
	ModuleEnums  ModuleKind = "enums"
	ModuleModels ModuleKind = "models"
)

type Module struct {
	Kind    ModuleKind
	Path    string
	Imports []Import
	Enums   []Enum
	Models  []Model
	Exports []string
}

type Import struct {
	Group  ImportGroup
	Module string
	Names  []string
}

type ImportGroup string

const (
	ImportFuture   ImportGroup = "future"
	ImportStandard ImportGroup = "standard"
	ImportLocal    ImportGroup = "local"
)

type Enum struct {
	Name    string
	Members []EnumMember
}

type EnumMember struct {
	Name  string
	Value string
}

type Model struct {
	Name   string
	Fields []Field
}

type Field struct {
	Name     string
	JSONName string
	Type     string
	Required bool
}
