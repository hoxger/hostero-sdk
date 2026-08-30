package python

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

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
	aliases, aliasDependencies, err := buildAliases(source.Aliases, symbols)
	if err != nil {
		return Document{}, err
	}
	models, modelDependencies, err := buildModels(source.Models, symbols)
	if err != nil {
		return Document{}, err
	}
	operations := buildOperations(source.Operations)
	services, serviceRefs, err := buildServices(source.Operations, symbols)
	if err != nil {
		return Document{}, err
	}

	exports := make([]string, 0, len(enums)+len(models)+len(aliases)+len(services)+3)
	for _, enum := range enums {
		exports = append(exports, enum.Name)
	}
	for _, model := range models {
		exports = append(exports, model.Name)
	}
	exports = append(exports, "RedirectResponse")
	for _, alias := range aliases {
		exports = append(exports, alias.Name)
	}
	for _, service := range services {
		exports = append(exports, service.Name)
	}
	exports = append(exports, "OPERATIONS", "Operation")
	sort.Strings(exports)

	modules := []Module{
		{
			Kind:    ModuleInit,
			Path:    "__init__.py",
			Imports: initModuleImports(enums, models, aliases, services),
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
	}
	if len(aliases) != 0 {
		modules = append(modules, Module{
			Kind:    ModuleTypes,
			Path:    "types.py",
			Imports: typeModuleImports(aliasDependencies),
			Aliases: aliases,
		})
	}
	modules = append(modules, Module{
		Kind:       ModuleOperations,
		Path:       "operations.py",
		Imports:    operationModuleImports(),
		Operations: operations,
	})
	if len(services) != 0 {
		modules = append(modules, Module{
			Kind:     ModuleServices,
			Path:     "services.py",
			Imports:  serviceModuleImports(serviceRefs),
			Services: services,
		})
	}
	return Document{Modules: modules}, nil
}

func buildOperations(source []contract.Operation) []Operation {
	operations := make([]Operation, 0, len(source))
	for _, sourceOperation := range source {
		operations = append(operations, Operation{
			ID:          sourceOperation.ID,
			Method:      sourceOperation.Method,
			Path:        sourceOperation.Path,
			Permissions: append([]string(nil), sourceOperation.Permissions...),
			TargetKinds: append([]string(nil), sourceOperation.TargetKinds...),
		})
	}
	return operations
}

type symbols struct {
	models  map[string]string
	enums   map[string]string
	aliases map[string]string
}

func buildSymbols(source contract.Document) (symbols, error) {
	result := symbols{
		models:  make(map[string]string, len(source.Models)),
		enums:   make(map[string]string, len(source.Enums)),
		aliases: make(map[string]string, len(source.Aliases)),
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
	for _, alias := range source.Aliases {
		name, err := className(alias.Name)
		if err != nil {
			return symbols{}, fmt.Errorf("alias %q: %w", alias.Name, err)
		}
		if err := reserveSymbol(seen, name, "alias", alias.Name); err != nil {
			return symbols{}, err
		}
		result.aliases[alias.Name] = name
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
		enum := Enum{Name: name, Docstring: sourceEnum.Description}
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
	aliasNames          []string
	needsAnnotations    bool
	needsDataclassField bool
	needsAny            bool
}

type typeReferences struct {
	models           map[string]struct{}
	enums            map[string]struct{}
	aliases          map[string]struct{}
	usesAny          bool
	usesUpload       bool
	usesBuiltins     bool
	referencesModels bool
}

func (references *typeReferences) merge(other typeReferences) {
	if references.models == nil {
		references.models = make(map[string]struct{})
	}
	for name := range other.models {
		references.models[name] = struct{}{}
	}
	if references.enums == nil {
		references.enums = make(map[string]struct{})
	}
	for name := range other.enums {
		references.enums[name] = struct{}{}
	}
	if references.aliases == nil {
		references.aliases = make(map[string]struct{})
	}
	for name := range other.aliases {
		references.aliases[name] = struct{}{}
	}
	references.usesAny = references.usesAny || other.usesAny
	references.usesUpload = references.usesUpload || other.usesUpload
	references.usesBuiltins = references.usesBuiltins || other.usesBuiltins
	references.referencesModels = references.referencesModels || other.referencesModels
}

func (references typeReferences) modelNames() []string {
	return sortedNames(references.models)
}

func (references typeReferences) enumNames() []string {
	return sortedNames(references.enums)
}

func (references typeReferences) aliasNames() []string {
	return sortedNames(references.aliases)
}

func buildModels(source []contract.Model, symbols symbols) ([]Model, modelDependencies, error) {
	models := make([]Model, 0, len(source))
	used := typeReferences{}
	dependencies := modelDependencies{}
	for _, sourceModel := range source {
		name, found := symbols.models[sourceModel.Name]
		if !found {
			return nil, modelDependencies{}, fmt.Errorf("model %q has no Python symbol", sourceModel.Name)
		}
		seenFields := make(map[string]string, len(sourceModel.Fields))
		model := Model{Name: name, Docstring: sourceModel.Description}
		for _, sourceField := range sourceModel.Fields {
			field, references, err := buildField(sourceField, symbols)
			if err != nil {
				return nil, modelDependencies{}, fmt.Errorf("model %q field %q: %w", sourceModel.Name, sourceField.Name, err)
			}
			if existing, found := seenFields[field.Name]; found {
				return nil, modelDependencies{}, fmt.Errorf("model %q field %q maps to Python field %q and collides with %q", sourceModel.Name, sourceField.Name, field.Name, existing)
			}
			seenFields[field.Name] = sourceField.Name
			dependencies.needsDataclassField = dependencies.needsDataclassField || field.Name != field.JSONName
			dependencies.needsAnnotations = dependencies.needsAnnotations || references.referencesModels
			dependencies.needsAny = dependencies.needsAny || references.usesAny
			used.merge(references)
			model.Fields = append(model.Fields, field)
		}
		models = append(models, model)
	}

	dependencies.enumNames = used.enumNames()
	dependencies.aliasNames = used.aliasNames()
	return models, dependencies, nil
}

func buildAliases(source []contract.Alias, symbols symbols) ([]Alias, typeReferences, error) {
	aliases := make([]Alias, 0, len(source))
	dependencies := typeReferences{}
	for _, sourceAlias := range source {
		name, found := symbols.aliases[sourceAlias.Name]
		if !found {
			return nil, typeReferences{}, fmt.Errorf("alias %q has no Python symbol", sourceAlias.Name)
		}
		typeName, references, err := buildType(sourceAlias.Type, symbols)
		if err != nil {
			return nil, typeReferences{}, fmt.Errorf("alias %q: %w", sourceAlias.Name, err)
		}
		aliases = append(aliases, Alias{
			Name:       name,
			Type:       typeName,
			QuotedType: len(references.aliases) != 0,
		})
		dependencies.merge(references)
	}
	return aliases, dependencies, nil
}

func buildServices(source []contract.Operation, symbols symbols) ([]ServiceClass, typeReferences, error) {
	type rawNode struct {
		group      []string
		className  string
		children   map[string]*rawNode
		operations []contract.Operation
	}

	root := &rawNode{children: make(map[string]*rawNode)}

	for _, op := range source {
		current := root
		for i, segment := range op.ClientMetadata.Group {
			if current.children[segment] == nil {
				subGroup := op.ClientMetadata.Group[:i+1]
				cName, err := serviceClassName(subGroup)
				if err != nil {
					return nil, typeReferences{}, err
				}
				current.children[segment] = &rawNode{
					group:     subGroup,
					className: cName,
					children:  make(map[string]*rawNode),
				}
			}
			current = current.children[segment]
		}
		current.operations = append(current.operations, op)
	}

	var classes []ServiceClass
	serviceRefs := typeReferences{}

	var visit func(node *rawNode) error
	visit = func(node *rawNode) error {
		childNames := make([]string, 0, len(node.children))
		for name := range node.children {
			childNames = append(childNames, name)
		}
		sort.Strings(childNames)

		for _, childName := range childNames {
			if err := visit(node.children[childName]); err != nil {
				return err
			}
		}

		var subServices []ServiceProperty
		for _, childName := range childNames {
			child := node.children[childName]
			propName, err := groupFieldName(childName)
			if err != nil {
				return err
			}
			subServices = append(subServices, ServiceProperty{
				Name:      propName,
				ClassName: child.className,
			})
		}

		var methods []ServiceMethod
		for _, op := range node.operations {
			method, refs, err := buildServiceMethod(op, symbols)
			if err != nil {
				return err
			}
			serviceRefs.merge(refs)
			methods = append(methods, method)
		}

		name := node.className
		if len(node.group) == 0 {
			name = "_GeneratedServicesMixin"
		}

		classes = append(classes, ServiceClass{
			Name:        name,
			SubServices: subServices,
			Methods:     methods,
		})
		return nil
	}

	if err := visit(root); err != nil {
		return nil, typeReferences{}, err
	}

	return classes, serviceRefs, nil
}

func buildServiceMethod(op contract.Operation, symbols symbols) (ServiceMethod, typeReferences, error) {
	references := typeReferences{
		models:  make(map[string]struct{}),
		enums:   make(map[string]struct{}),
		aliases: make(map[string]struct{}),
	}
	name, err := methodName(op.ClientMetadata.Method)
	if err != nil {
		return ServiceMethod{}, typeReferences{}, err
	}

	pathParamsMap := make(map[string]contract.Parameter)
	for _, param := range op.Parameters {
		if param.Location == contract.ParameterPath {
			pathParamsMap[param.Name] = param
		}
	}

	pathExpr, err := formatPathExpr(op.Path, pathParamsMap)
	if err != nil {
		return ServiceMethod{}, typeReferences{}, err
	}
	pathParamNames, err := extractPathParameterNames(op.Path)
	if err != nil {
		return ServiceMethod{}, typeReferences{}, err
	}
	pathParams := make([]MethodParam, 0, len(pathParamNames))
	for _, pName := range pathParamNames {
		param, found := pathParamsMap[pName]
		if !found {
			return ServiceMethod{}, typeReferences{}, fmt.Errorf("path parameter %q is not defined in operation parameters", pName)
		}
		pyName, err := fieldName(param.Name)
		if err != nil {
			return ServiceMethod{}, typeReferences{}, err
		}
		pathParams = append(pathParams, MethodParam{
			Name:        pyName,
			JSONName:    param.Name,
			Description: param.Description,
			Type:        "str",
			Required:    true,
		})
	}

	queryParams := make([]MethodParam, 0)
	for _, param := range op.Parameters {
		if param.Location == contract.ParameterQuery {
			pName, err := fieldName(param.Name)
			if err != nil {
				return ServiceMethod{}, typeReferences{}, err
			}
			pType, pRefs, err := buildServiceType(param.Type, symbols)
			if err != nil {
				return ServiceMethod{}, typeReferences{}, err
			}
			references.merge(pRefs)
			if !param.Required && !param.Type.Nullable {
				pType += " | None"
			}
			queryParams = append(queryParams, MethodParam{
				Name:        pName,
				JSONName:    param.Name,
				Description: param.Description,
				Type:        pType,
				Required:    param.Required,
			})
		}
	}

	hasBody := false
	var bodyParam *MethodParam
	isRawBody := false
	isMultipart := false

	if op.RequestBody != nil {
		hasBody = true
		if op.RequestBody.ContentType == "multipart/form-data" {
			isMultipart = true
			references.usesUpload = true
			references.usesAny = true
		} else if op.RequestBody.ContentType == "application/json" {
			bType, bRefs, err := buildServiceType(op.RequestBody.Type, symbols)
			if err != nil {
				return ServiceMethod{}, typeReferences{}, err
			}
			references.merge(bRefs)
			references.usesAny = true
			bodyType := bType + " | dict[str, Any]"
			if !op.RequestBody.Required {
				bodyType += " | None"
			}
			bodyParam = &MethodParam{
				Name:     "body",
				Type:     bodyType,
				Required: op.RequestBody.Required,
			}
		} else {
			isRawBody = true
		}
	}

	returnType := "None"
	returnModelName := ""
	isReturnList := false
	isReturnModel := false

	if op.Success.Status == 302 {
		returnType = "RedirectResponse"
		references.models["RedirectResponse"] = struct{}{}
	} else if op.Success.Type != nil {
		tName, tRefs, err := buildServiceType(*op.Success.Type, symbols)
		if err != nil {
			return ServiceMethod{}, typeReferences{}, err
		}
		references.merge(tRefs)
		returnType = tName
		if op.Success.Type.Kind == contract.KindModel {
			isReturnModel = true
			returnModelName = symbols.models[op.Success.Type.Name]
			references.models[returnModelName] = struct{}{}
		} else if op.Success.Type.Kind == contract.KindArray && op.Success.Type.Items != nil && op.Success.Type.Items.Kind == contract.KindModel {
			isReturnList = true
			returnModelName = symbols.models[op.Success.Type.Items.Name]
			references.models[returnModelName] = struct{}{}
			references.usesBuiltins = true
		}
	}

	docstring := buildMethodDocstring(op, pathParams, queryParams, hasBody, isMultipart)

	return ServiceMethod{
		Name:            name,
		Docstring:       docstring,
		OperationID:     op.ID,
		HTTPMethod:      op.Method,
		PathExpr:        pathExpr,
		PathParams:      pathParams,
		QueryParams:     queryParams,
		HasBody:         hasBody,
		BodyParam:       bodyParam,
		IsRawBody:       isRawBody,
		IsMultipart:     isMultipart,
		SuccessStatus:   op.Success.Status,
		ReturnType:      returnType,
		ReturnModelName: returnModelName,
		IsReturnList:    isReturnList,
		IsReturnModel:   isReturnModel,
	}, references, nil
}

func buildMethodDocstring(op contract.Operation, pathParams []MethodParam, queryParams []MethodParam, hasBody bool, isMultipart bool) string {
	var sections []string

	description := strings.TrimSpace(op.Description)
	if description != "" {
		sections = append(sections, normalizeSentence(description))
	} else if op.Summary != "" {
		sections = append(sections, normalizeSentence(op.Summary))
	}

	if len(op.Permissions) > 0 {
		var permLines []string
		permLines = append(permLines, "Required permissions:")
		for _, perm := range op.Permissions {
			permLines = append(permLines, fmt.Sprintf("    - `%s`", perm))
		}
		sections = append(sections, strings.Join(permLines, "\n"))
	}

	var argLines []string
	for _, p := range pathParams {
		if p.Description != "" {
			argLines = append(argLines, fmt.Sprintf("    %s: %s", p.Name, p.Description))
		}
	}
	for _, q := range queryParams {
		if q.Description != "" {
			argLines = append(argLines, fmt.Sprintf("    %s: %s", q.Name, q.Description))
		}
	}
	if len(argLines) > 0 {
		sections = append(sections, "Args:\n"+strings.Join(argLines, "\n"))
	}

	return strings.Join(sections, "\n\n")
}

func normalizeSentence(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	allUpperInitials := true
	for _, w := range words {
		runes := []rune(w)
		if len(runes) > 0 && !unicode.IsUpper(runes[0]) && !unicode.IsDigit(runes[0]) {
			allUpperInitials = false
			break
		}
	}
	if allUpperInitials && len(words) > 1 {
		for i := 1; i < len(words); i++ {
			if !isPreservedDocWord(words[i]) {
				words[i] = strings.ToLower(words[i])
			}
		}
	}
	result := strings.Join(words, " ")
	if !strings.HasSuffix(result, ".") {
		result += "."
	}
	return result
}

func isPreservedDocWord(w string) bool {
	clean := strings.Trim(w, ".,!?:;\"'()[]{}")
	if len(clean) > 1 && strings.ToUpper(clean) == clean {
		return true
	}
	switch strings.ToLower(clean) {
	case "minecraft", "axon", "mcjars", "steam", "curseforge", "modrinth", "pterodactyl":
		return true
	}
	return false
}

func buildServiceType(source contract.Type, symbols symbols) (string, typeReferences, error) {
	var typeName string
	references := typeReferences{
		models:  make(map[string]struct{}),
		enums:   make(map[string]struct{}),
		aliases: make(map[string]struct{}),
	}
	switch source.Kind {
	case contract.KindString:
		typeName = "str"
	case contract.KindInteger:
		typeName = "int"
	case contract.KindNumber:
		typeName = "float"
	case contract.KindBoolean:
		typeName = "bool"
	case contract.KindAny:
		typeName = "Any"
		references.usesAny = true
	case contract.KindArray:
		if source.Items == nil {
			return "", typeReferences{}, fmt.Errorf("array items are required")
		}
		itemType, itemReferences, err := buildServiceType(*source.Items, symbols)
		if err != nil {
			return "", typeReferences{}, fmt.Errorf("array items: %w", err)
		}
		typeName = "builtins.list[" + itemType + "]"
		references.usesBuiltins = true
		references.merge(itemReferences)
	case contract.KindMap:
		if source.Items == nil {
			return "", typeReferences{}, fmt.Errorf("map values are required")
		}
		valueType, valueReferences, err := buildServiceType(*source.Items, symbols)
		if err != nil {
			return "", typeReferences{}, fmt.Errorf("map values: %w", err)
		}
		typeName = "dict[str, " + valueType + "]"
		references.merge(valueReferences)
	case contract.KindUnion:
		if len(source.Values) == 0 {
			return "", typeReferences{}, fmt.Errorf("union values are required")
		}
		values := make([]string, 0, len(source.Values))
		for _, value := range source.Values {
			valueType, valueReferences, err := buildServiceType(value, symbols)
			if err != nil {
				return "", typeReferences{}, fmt.Errorf("union value: %w", err)
			}
			values = append(values, valueType)
			references.merge(valueReferences)
		}
		typeName = strings.Join(values, " | ")
	case contract.KindModel:
		name, found := symbols.models[source.Name]
		if !found {
			return "", typeReferences{}, fmt.Errorf("references unknown model %q", source.Name)
		}
		typeName = name
		references.models[name] = struct{}{}
		references.referencesModels = true
	case contract.KindEnum:
		name, found := symbols.enums[source.Name]
		if !found {
			return "", typeReferences{}, fmt.Errorf("references unknown enum %q", source.Name)
		}
		typeName = name
		references.enums[name] = struct{}{}
	case contract.KindAlias:
		name, found := symbols.aliases[source.Name]
		if !found {
			return "", typeReferences{}, fmt.Errorf("references unknown alias %q", source.Name)
		}
		typeName = name
		references.aliases[name] = struct{}{}
	default:
		return "", typeReferences{}, fmt.Errorf("unsupported contract type %q", source.Kind)
	}
	if source.Nullable {
		typeName += " | None"
	}
	return typeName, references, nil
}

func formatPathExpr(path string, pathParamsMap map[string]contract.Parameter) (string, error) {
	paramNames, err := extractPathParameterNames(path)
	if err != nil {
		return "", err
	}
	if len(paramNames) == 0 {
		return strconv.Quote(path), nil
	}

	var builder strings.Builder
	lastIndex := 0
	for {
		start := strings.IndexByte(path[lastIndex:], '{')
		if start == -1 {
			builder.WriteString(path[lastIndex:])
			break
		}
		start += lastIndex
		end := strings.IndexByte(path[start:], '}')
		if end == -1 {
			return "", fmt.Errorf("unclosed placeholder in path %q", path)
		}
		end += start
		builder.WriteString(path[lastIndex:start])

		paramName := path[start+1 : end]
		param, found := pathParamsMap[paramName]
		if !found {
			return "", fmt.Errorf("path parameter %q is not defined in operation parameters", paramName)
		}
		pyName, err := fieldName(param.Name)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(&builder, "{_quote_path(str(%s), safe='')}", pyName)
		lastIndex = end + 1
	}

	return fmt.Sprintf("f%q", builder.String()), nil
}

func extractPathParameterNames(path string) ([]string, error) {
	var names []string
	seen := make(map[string]struct{})
	i := 0
	for i < len(path) {
		start := strings.IndexByte(path[i:], '{')
		if start == -1 {
			break
		}
		start += i
		end := strings.IndexByte(path[start:], '}')
		if end == -1 {
			return nil, fmt.Errorf("unclosed placeholder in path %q", path)
		}
		end += start
		paramName := path[start+1 : end]
		if paramName == "" {
			return nil, fmt.Errorf("empty placeholder in path %q", path)
		}
		if _, exists := seen[paramName]; !exists {
			seen[paramName] = struct{}{}
			names = append(names, paramName)
		}
		i = end + 1
	}
	return names, nil
}

func enumModuleImports(enums []Enum) []Import {
	if len(enums) == 0 {
		return nil
	}
	return []Import{{Group: ImportStandard, Module: "enum", Names: []string{"Enum"}}}
}

func operationModuleImports() []Import {
	return []Import{
		{Group: ImportFuture, Module: "__future__", Names: []string{"annotations"}},
		{Group: ImportStandard, Module: "dataclasses", Names: []string{"dataclass"}},
		{Group: ImportStandard, Module: "typing", Names: []string{"Final"}},
	}
}

func modelModuleImports(models []Model, dependencies modelDependencies) []Import {
	imports := make([]Import, 0, 5)
	imports = append(imports, Import{Group: ImportFuture, Module: "__future__", Names: []string{"annotations"}})
	imports = append(imports, Import{Group: ImportStandard, Module: "collections.abc", Names: []string{"Mapping"}})
	if len(models) != 0 {
		names := []string{"dataclass"}
		if dependencies.needsDataclassField {
			names = append(names, "field")
		}
		imports = append(imports, Import{Group: ImportStandard, Module: "dataclasses", Names: names})
	}
	imports = append(imports, Import{Group: ImportStandard, Module: "typing", Names: []string{"Any"}})
	if len(dependencies.enumNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".enums", Names: dependencies.enumNames})
	}
	if len(dependencies.aliasNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".types", Names: dependencies.aliasNames})
	}
	return imports
}

func typeModuleImports(dependencies typeReferences) []Import {
	names := []string{"TypeAlias"}
	if dependencies.usesAny {
		names = append([]string{"Any"}, names...)
	}
	imports := []Import{
		{Group: ImportFuture, Module: "__future__", Names: []string{"annotations"}},
		{Group: ImportStandard, Module: "typing", Names: names},
	}
	if enumNames := dependencies.enumNames(); len(enumNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".enums", Names: enumNames})
	}
	if modelNames := dependencies.modelNames(); len(modelNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".models", Names: modelNames})
	}
	return imports
}

func serviceModuleImports(refs typeReferences) []Import {
	imports := []Import{
		{Group: ImportFuture, Module: "__future__", Names: []string{"annotations"}},
	}
	if refs.usesBuiltins {
		imports = append(imports, Import{Group: ImportStandard, Module: "builtins", Names: nil})
	}
	typingNames := make([]string, 0, 2)
	if refs.usesAny {
		typingNames = append(typingNames, "Any")
	}
	if len(typingNames) != 0 {
		imports = append(imports, Import{Group: ImportStandard, Module: "typing", Names: typingNames})
	}
	imports = append(imports, Import{Group: ImportStandard, Module: "urllib.parse", Names: []string{"quote as _quote_path"}})

	if enumNames := refs.enumNames(); len(enumNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".enums", Names: enumNames})
	}
	if modelNames := refs.modelNames(); len(modelNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".models", Names: modelNames})
	}
	imports = append(imports, Import{Group: ImportLocal, Module: ".operations", Names: []string{"OPERATIONS"}})
	if aliasNames := refs.aliasNames(); len(aliasNames) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".types", Names: aliasNames})
	}
	if refs.usesUpload {
		imports = append(imports, Import{Group: ImportLocal, Module: ".._upload", Names: []string{"Upload"}})
	}
	if _, hasRedirect := refs.models["RedirectResponse"]; hasRedirect {
		imports = append(imports, Import{Group: ImportLocal, Module: "..exceptions", Names: []string{"ApiError"}})
	}
	return imports
}

func initModuleImports(enums []Enum, models []Model, aliases []Alias, services []ServiceClass) []Import {
	imports := make([]Import, 0, 5)
	if names := namesOfEnums(enums); len(names) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".enums", Names: names})
	}
	modelNames := namesOfModels(models)
	modelNames = append(modelNames, "RedirectResponse")
	sort.Strings(modelNames)
	imports = append(imports, Import{Group: ImportLocal, Module: ".models", Names: modelNames})
	if names := namesOfAliases(aliases); len(names) != 0 {
		imports = append(imports, Import{Group: ImportLocal, Module: ".types", Names: names})
	}
	imports = append(imports, Import{Group: ImportLocal, Module: ".operations", Names: []string{"OPERATIONS", "Operation"}})
	if len(services) != 0 {
		serviceNames := make([]string, 0, len(services))
		for _, service := range services {
			serviceNames = append(serviceNames, service.Name)
		}
		sort.Strings(serviceNames)
		imports = append(imports, Import{Group: ImportLocal, Module: ".services", Names: serviceNames})
	}
	return imports
}

func buildField(source contract.Field, symbols symbols) (Field, typeReferences, error) {
	name, err := fieldName(source.Name)
	if err != nil {
		return Field{}, typeReferences{}, err
	}
	typeName, references, err := buildType(source.Type, symbols)
	if err != nil {
		return Field{}, typeReferences{}, err
	}
	if !source.Required && !source.Type.Nullable {
		typeName += " | None"
	}

	codecKind := CodecPrimitive
	targetType := ""

	switch source.Type.Kind {
	case contract.KindModel:
		codecKind = CodecModel
		targetType = symbols.models[source.Type.Name]
	case contract.KindEnum:
		codecKind = CodecEnum
		targetType = symbols.enums[source.Type.Name]
	case contract.KindAlias:
		codecKind = CodecAlias
		targetType = symbols.aliases[source.Type.Name]
	case contract.KindArray:
		if source.Type.Items != nil {
			if source.Type.Items.Kind == contract.KindModel {
				codecKind = CodecListModel
				targetType = symbols.models[source.Type.Items.Name]
			} else if source.Type.Items.Kind == contract.KindEnum {
				codecKind = CodecListEnum
				targetType = symbols.enums[source.Type.Items.Name]
			} else {
				codecKind = CodecListPrim
			}
		}
	case contract.KindMap:
		if source.Type.Items != nil {
			if source.Type.Items.Kind == contract.KindModel {
				codecKind = CodecMapModel
				targetType = symbols.models[source.Type.Items.Name]
			} else if source.Type.Items.Kind == contract.KindEnum {
				codecKind = CodecMapEnum
				targetType = symbols.enums[source.Type.Items.Name]
			} else {
				codecKind = CodecMapPrim
			}
		}
	}

	return Field{
		Name:       name,
		JSONName:   source.Name,
		Type:       typeName,
		Required:   source.Required,
		Nullable:   source.Type.Nullable,
		CodecKind:  codecKind,
		TargetType: targetType,
	}, references, nil
}

func buildType(source contract.Type, symbols symbols) (string, typeReferences, error) {
	var typeName string
	references := typeReferences{}
	switch source.Kind {
	case contract.KindString:
		typeName = "str"
	case contract.KindInteger:
		typeName = "int"
	case contract.KindNumber:
		typeName = "float"
	case contract.KindBoolean:
		typeName = "bool"
	case contract.KindAny:
		typeName = "Any"
		references.usesAny = true
	case contract.KindArray:
		if source.Items == nil {
			return "", typeReferences{}, fmt.Errorf("array items are required")
		}
		itemType, itemReferences, err := buildType(*source.Items, symbols)
		if err != nil {
			return "", typeReferences{}, fmt.Errorf("array items: %w", err)
		}
		typeName = "list[" + itemType + "]"
		references.merge(itemReferences)
	case contract.KindMap:
		if source.Items == nil {
			return "", typeReferences{}, fmt.Errorf("map values are required")
		}
		valueType, valueReferences, err := buildType(*source.Items, symbols)
		if err != nil {
			return "", typeReferences{}, fmt.Errorf("map values: %w", err)
		}
		typeName = "dict[str, " + valueType + "]"
		references.merge(valueReferences)
	case contract.KindUnion:
		if len(source.Values) == 0 {
			return "", typeReferences{}, fmt.Errorf("union values are required")
		}
		values := make([]string, 0, len(source.Values))
		for _, value := range source.Values {
			valueType, valueReferences, err := buildType(value, symbols)
			if err != nil {
				return "", typeReferences{}, fmt.Errorf("union value: %w", err)
			}
			values = append(values, valueType)
			references.merge(valueReferences)
		}
		typeName = strings.Join(values, " | ")
	case contract.KindModel:
		name, found := symbols.models[source.Name]
		if !found {
			return "", typeReferences{}, fmt.Errorf("references unknown model %q", source.Name)
		}
		typeName = name
		references.referencesModels = true
	case contract.KindEnum:
		name, found := symbols.enums[source.Name]
		if !found {
			return "", typeReferences{}, fmt.Errorf("references unknown enum %q", source.Name)
		}
		typeName = name
		references.enums = map[string]struct{}{name: {}}
	case contract.KindAlias:
		name, found := symbols.aliases[source.Name]
		if !found {
			return "", typeReferences{}, fmt.Errorf("references unknown alias %q", source.Name)
		}
		typeName = name
		references.aliases = map[string]struct{}{name: {}}
	default:
		return "", typeReferences{}, fmt.Errorf("unsupported contract type %q", source.Kind)
	}
	if source.Nullable {
		typeName += " | None"
	}
	return typeName, references, nil
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

func namesOfAliases(aliases []Alias) []string {
	names := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		names = append(names, alias.Name)
	}
	return names
}

func sortedNames(names map[string]struct{}) []string {
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
