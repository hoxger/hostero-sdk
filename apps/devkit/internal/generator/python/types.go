package python

type Document struct {
	Modules []Module
}

type GenerationMetadata struct {
	DevKitVersion string
	OpenAPISource string
	Release       string
	SHA256        string
}

type ModuleKind string

const (
	ModuleInit       ModuleKind = "init"
	ModuleEnums      ModuleKind = "enums"
	ModuleModels     ModuleKind = "models"
	ModuleOperations ModuleKind = "operations"
	ModuleTypes      ModuleKind = "types"
)

type Module struct {
	Kind       ModuleKind
	Path       string
	Imports    []Import
	Enums      []Enum
	Models     []Model
	Operations []Operation
	Aliases    []Alias
	Exports    []string
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

type Operation struct {
	ID          string
	Method      string
	Path        string
	Permissions []string
	TargetKinds []string
}

type Alias struct {
	Name       string
	Type       string
	QuotedType bool
}

type Field struct {
	Name     string
	JSONName string
	Type     string
	Required bool
}
