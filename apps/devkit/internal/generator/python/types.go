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
	ModuleResources  ModuleKind = "resources"
	ModuleTypes      ModuleKind = "types"
	ModuleServices   ModuleKind = "services"
)

type Module struct {
	Kind       ModuleKind
	Path       string
	Imports    []Import
	Enums      []Enum
	Models     []Model
	Operations []Operation
	Aliases    []Alias
	Services   []ServiceClass
	Resources  []Resource
	Pages      []ResourcePage
	Exports    []string
}

type Resource struct {
	Name         string
	ModelName    string
	Fields       []Field
	BoundMethods []string
}

type ResourcePage struct {
	Name      string
	ModelName string
	ItemName  string
	Fields    []Field
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
	Name      string
	Docstring string
	Members   []EnumMember
}

type EnumMember struct {
	Name  string
	Value string
}

type Model struct {
	Name      string
	Docstring string
	Fields    []Field
}

type FieldCodecKind string

const (
	CodecPrimitive FieldCodecKind = "primitive"
	CodecEnum      FieldCodecKind = "enum"
	CodecModel     FieldCodecKind = "model"
	CodecAlias     FieldCodecKind = "alias"
	CodecListModel FieldCodecKind = "list_model"
	CodecListEnum  FieldCodecKind = "list_enum"
	CodecListPrim  FieldCodecKind = "list_prim"
	CodecMapModel  FieldCodecKind = "map_model"
	CodecMapPrim   FieldCodecKind = "map_prim"
	CodecMapEnum   FieldCodecKind = "map_enum"
)

type Field struct {
	Name       string
	JSONName   string
	Type       string
	Required   bool
	Nullable   bool
	CodecKind  FieldCodecKind
	TargetType string
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

type ServiceClass struct {
	Name        string
	SubServices []ServiceProperty
	Methods     []ServiceMethod
}

type ServiceProperty struct {
	Name      string
	ClassName string
}

type ServiceMethod struct {
	Name            string
	Docstring       string
	OperationID     string
	HTTPMethod      string
	PathExpr        string
	PathParams      []MethodParam
	QueryParams     []MethodParam
	HasBody         bool
	BodyParam       *MethodParam
	IsRawBody       bool
	IsMultipart     bool
	SuccessStatus   int
	ReturnType      string
	ReturnModelName string
	IsReturnList    bool
	IsReturnModel   bool
	ResourceName    string
	ResourcePage    string
}

type MethodParam struct {
	Name         string
	JSONName     string
	Description  string
	Type         string
	DefaultValue string
	Required     bool
}
