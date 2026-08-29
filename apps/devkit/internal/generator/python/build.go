package python

import (
	"fmt"
	"sort"

	"github.com/hoxger/hostero-sdk/apps/devkit/internal/contract"
)

func Build(source contract.Document) (Document, error) {
	symbols, err := buildSymbols(source)
	if err != nil {
		return Document{}, err
	}

	enums, err := buildEnums(source.Enums)
	if err != nil {
		return Document{}, err
	}
	models, modelDependencies, err := buildModels(source.Models, symbols)
	if err != nil {
		return Document{}, err
	}

	exports := make([]string, 0, len(enums)+len(models))
	for _, enum := range enums {
		exports = append(exports, enum.Name)
	}
	for _, model := range models {
		exports = append(exports, model.Name)
	}
	sort.Strings(exports)

	return Document{Modules: []Module{
		{
			Kind: ModuleInit,
			Path: "__init__.py",
			Imports: []Import{
				{Group: ImportLocal, Module: ".enums", Names: namesOfEnums(enums)},
				{Group: ImportLocal, Module: ".models", Names: namesOfModels(models)},
			},
			Exports: exports,
		},
		{
			Kind:    ModuleEnums,
			Path:    "enums.py",
			Imports: enumModuleImports(enums),
			Enums:   enums,
		},
		{
			Kind:    ModuleModels,
			Path:    "models.py",
			Imports: modelModuleImports(models, modelDependencies),
			Models:  models,
		},
	}}, nil
}

type symbols struct {
	models map[string]string
	enums  map[string]string
}

func buildSymbols(source contract.Document) (symbols, error) {
	result := symbols{
		models: make(map[string]string, len(source.Models)),
		enums:  make(map[string]string, len(source.Enums)),
	}
	seen := make(map[string]symbolOrigin, len(source.Models)+len(source.Enums))

	for _, model := range source.Models {
		name, err := className(model.Name)
		if err != nil {
			return symbols{}, fmt.Errorf("model %q: %w", model.Name, err)
		}
		if err := reserveSymbol(seen, name, "model", model.Name); err != nil {
			return symbols{}, err
		}
		result.models[model.Name] = name
	}
	for _, enum := range source.Enums {
		name, err := className(enum.Name)
		if err != nil {
			return symbols{}, fmt.Errorf("enum %q: %w", enum.Name, err)
		}
		if err := reserveSymbol(seen, name, "enum", enum.Name); err != nil {
			return symbols{}, err
		}
		result.enums[enum.Name] = name
	}

	return result, nil
}

type symbolOrigin struct {
	kind   string
	source string
}

func reserveSymbol(seen map[string]symbolOrigin, name string, kind string, source string) error {
	if existing, found := seen[name]; found {
		return fmt.Errorf("python %s name %q for %q collides with %s %q", kind, name, source, existing.kind, existing.source)
	}
	seen[name] = symbolOrigin{kind: kind, source: source}
	return nil
}

func buildEnums(source []contract.Enum) ([]Enum, error) {
	enums := make([]Enum, 0, len(source))
	for _, sourceEnum := range source {
		name, err := className(sourceEnum.Name)
		if err != nil {
			return nil, fmt.Errorf("enum %q: %w", sourceEnum.Name, err)
		}

		seen := make(map[string]string, len(sourceEnum.Values))
		enum := Enum{Name: name}
		for _, value := range sourceEnum.Values {
			memberName, err := enumMemberName(value)
			if err != nil {
				return nil, fmt.Errorf("enum %q value %q: %w", sourceEnum.Name, value, err)
			}
			if existing, found := seen[memberName]; found {
				return nil, fmt.Errorf("enum %q value %q maps to Python member %q already used by %q", sourceEnum.Name, value, memberName, existing)
			}
			seen[memberName] = value
			enum.Members = append(enum.Members, EnumMember{Name: memberName, Value: value})
		}
		enums = append(enums, enum)
	}
	return enums, nil
}

type modelDependencies struct {
	enumNames           []string
	needsAnnotations    bool
	needsDataclassField bool
}

func buildModels(source []contract.Model, symbols symbols) ([]Model, modelDependencies, error) {
	models := make([]Model, 0, len(source))
	usedEnums := make(map[string]struct{})
	dependencies := modelDependencies{}
	for _, sourceModel := range source {
		name, found := symbols.models[sourceModel.Name]
		if !found {
			return nil, modelDependencies{}, fmt.Errorf("model %q has no Python symbol", sourceModel.Name)
		}
		seenFields := make(map[string]string, len(sourceModel.Fields))
		model := Model{Name: name}
		for _, sourceField := range sourceModel.Fields {
			field, enumNames, err := buildField(sourceField, symbols)
			if err != nil {
				return nil, modelDependencies{}, fmt.Errorf("model %q field %q: %w", sourceModel.Name, sourceField.Name, err)
			}
			if existing, found := seenFields[field.Name]; found {
				return nil, modelDependencies{}, fmt.Errorf("model %q field %q maps to Python field %q and collides with %q", sourceModel.Name, sourceField.Name, field.Name, existing)
			}
			seenFields[field.Name] = sourceField.Name
			dependencies.needsDataclassField = dependencies.needsDataclassField || field.Name != field.JSONName
			dependencies.needsAnnotations = dependencies.needsAnnotations || referencesModel(sourceField.Type)
			for _, enumName := range enumNames {
				usedEnums[enumName] = struct{}{}
			}
			model.Fields = append(model.Fields, field)
		}
		models = append(models, model)
	}

	enumNames := make([]string, 0, len(usedEnums))
	for enumName := range usedEnums {
		enumNames = append(enumNames, enumName)
	}
	sort.Strings(enumNames)

	dependencies.enumNames = enumNames
	return models, dependencies, nil
}

func enumModuleImports(enums []Enum) []Import {
	if len(enums) == 0 {
		return nil
	}
	return []Import{{Group: ImportStandard, Module: "enum", Names: []string{"Enum"}}}
}

func modelModuleImports(models []Model, dependencies modelDependencies) []Import {
	imports := make([]Import, 0, 3)
	if dependencies.needsAnnotations {
		imports = append(imports, Import{Group: ImportFuture, Module: "__future__", Names: []string{"annotations"}})
	}
	if len(models) != 0 {
		names := []string{"dataclass"}
		if dependencies.needsDataclassField {
			names = append(names, "field")
		}
		imports = append(imports, Import{Group: ImportStandard, Module: "dataclasses", Names: names})
	}
	if len(dependencies.enumNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".enums", Names: dependencies.enumNames})
	}
	return imports
}

func buildField(source contract.Field, symbols symbols) (Field, []string, error) {
	name, err := fieldName(source.Name)
	if err != nil {
		return Field{}, nil, err
	}
	typeName, enumNames, err := buildType(source.Type, symbols)
	if err != nil {
		return Field{}, nil, err
	}
	if !source.Required && !source.Type.Nullable {
		typeName += " | None"
	}
	return Field{Name: name, JSONName: source.Name, Type: typeName, Required: source.Required}, enumNames, nil
}

func buildType(source contract.Type, symbols symbols) (string, []string, error) {
	var typeName string
	var enumNames []string
	switch source.Kind {
	case contract.KindString:
		typeName = "str"
	case contract.KindInteger:
		typeName = "int"
	case contract.KindNumber:
		typeName = "float"
	case contract.KindBoolean:
		typeName = "bool"
	case contract.KindArray:
		if source.Items == nil {
			return "", nil, fmt.Errorf("array items are required")
		}
		itemType, itemEnums, err := buildType(*source.Items, symbols)
		if err != nil {
			return "", nil, fmt.Errorf("array items: %w", err)
		}
		typeName = "list[" + itemType + "]"
		enumNames = itemEnums
	case contract.KindModel:
		name, found := symbols.models[source.Name]
		if !found {
			return "", nil, fmt.Errorf("references unknown model %q", source.Name)
		}
		typeName = name
	case contract.KindEnum:
		name, found := symbols.enums[source.Name]
		if !found {
			return "", nil, fmt.Errorf("references unknown enum %q", source.Name)
		}
		typeName = name
		enumNames = []string{name}
	default:
		return "", nil, fmt.Errorf("unsupported contract type %q", source.Kind)
	}
	if source.Nullable {
		typeName += " | None"
	}
	return typeName, enumNames, nil
}

func referencesModel(source contract.Type) bool {
	if source.Kind == contract.KindModel {
		return true
	}
	return source.Kind == contract.KindArray && source.Items != nil && referencesModel(*source.Items)
}

func namesOfEnums(enums []Enum) []string {
	names := make([]string, 0, len(enums))
	for _, enum := range enums {
		names = append(names, enum.Name)
	}
	return names
}

func namesOfModels(models []Model) []string {
	names := make([]string, 0, len(models))
	for _, model := range models {
		names = append(names, model.Name)
	}
	return names
}
