package contract

import (
	"fmt"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func classifyComponentSchemas(components *openapi3.Components) (map[string]Kind, error) {
	if components == nil {
		return nil, fmt.Errorf("OpenAPI components are required")
	}

	classes := make(map[string]Kind, len(components.Schemas))
	for _, name := range sortedSchemaNames(components.Schemas) {
		schema, err := componentSchema(name, components.Schemas[name])
		if err != nil {
			return nil, err
		}
		switch {
		case isStringEnum(schema):
			classes[name] = KindEnum
		case schema.Type.Is(openapi3.TypeObject):
			if isMapSchema(schema) {
				classes[name] = KindAlias
			} else {
				classes[name] = KindModel
			}
		default:
			classes[name] = KindAlias
		}
	}

	return classes, nil
}

func buildComponentSchemas(components *openapi3.Components, classes map[string]Kind) ([]Model, []Enum, []Alias, error) {
	var models []Model
	var enums []Enum
	var aliases []Alias
	for _, name := range sortedSchemaNames(components.Schemas) {
		schema, err := componentSchema(name, components.Schemas[name])
		if err != nil {
			return nil, nil, nil, err
		}

		switch classes[name] {
		case KindModel:
			model, err := buildModel(name, schema, classes)
			if err != nil {
				return nil, nil, nil, err
			}
			models = append(models, model)
		case KindEnum:
			enum, err := buildEnum(name, schema)
			if err != nil {
				return nil, nil, nil, err
			}
			enums = append(enums, enum)
		case KindAlias:
			typeValue, err := buildType(&openapi3.SchemaRef{Value: schema}, classes)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("alias %q: %w", name, err)
			}
			aliases = append(aliases, Alias{Name: name, Type: typeValue})
		default:
			return nil, nil, nil, fmt.Errorf("component schema %q has unknown class", name)
		}
	}
	return models, enums, aliases, nil
}

func buildModel(name string, schema *openapi3.Schema, classes map[string]Kind) (Model, error) {
	if err := validateObjectSchema(name, schema); err != nil {
		return Model{}, err
	}

	required := make(map[string]struct{}, len(schema.Required))
	for _, fieldName := range schema.Required {
		required[fieldName] = struct{}{}
	}

	model := Model{Name: name}
	for _, fieldName := range sortedSchemaNames(schema.Properties) {
		fieldType, err := buildType(schema.Properties[fieldName], classes)
		if err != nil {
			return Model{}, fmt.Errorf("model %q field %q: %w", name, fieldName, err)
		}
		_, isRequired := required[fieldName]
		model.Fields = append(model.Fields, Field{Name: fieldName, Required: isRequired, Type: fieldType})
	}
	return model, nil
}

func buildEnum(name string, schema *openapi3.Schema) (Enum, error) {
	if err := validateSchemaKeywords(schema); err != nil {
		return Enum{}, fmt.Errorf("enum %q: %w", name, err)
	}
	if !schema.Type.Is(openapi3.TypeString) || len(schema.Enum) == 0 {
		return Enum{}, fmt.Errorf("enum %q must define string values", name)
	}

	enum := Enum{Name: name}
	for _, value := range schema.Enum {
		stringValue, ok := value.(string)
		if !ok || stringValue == "" {
			return Enum{}, fmt.Errorf("enum %q must contain non-empty string values", name)
		}
		enum.Values = append(enum.Values, stringValue)
	}
	sort.Strings(enum.Values)
	return enum, nil
}

func buildType(reference *openapi3.SchemaRef, classes map[string]Kind) (Type, error) {
	if reference == nil {
		return Type{}, fmt.Errorf("schema is required")
	}
	if reference.Ref != "" {
		name, err := localSchemaReferenceName(reference.Ref)
		if err != nil {
			return Type{}, err
		}
		kind, ok := classes[name]
		if !ok {
			return Type{}, fmt.Errorf("references unknown component schema %q", name)
		}
		return Type{Kind: kind, Name: name}, nil
	}
	if reference.Value == nil {
		return Type{}, fmt.Errorf("schema has no value")
	}

	schema := reference.Value
	if len(schema.AnyOf) != 0 || len(schema.OneOf) != 0 {
		if len(schema.AnyOf) != 0 && len(schema.OneOf) != 0 {
			return Type{}, fmt.Errorf("schema cannot define both anyOf and oneOf")
		}
		if len(schema.AnyOf) != 0 {
			return buildUnion(schema.AnyOf, classes)
		}
		return buildUnion(schema.OneOf, classes)
	}
	if err := validateSchemaKeywords(schema); err != nil {
		return Type{}, err
	}
	if schema.Type == nil || len(schema.Type.Slice()) == 0 {
		return Type{Kind: KindAny}, nil
	}

	nullable, typeName, err := schemaType(schema)
	if err != nil {
		return Type{}, err
	}
	result := Type{Format: schema.Format, Nullable: nullable || schema.Nullable}
	switch typeName {
	case openapi3.TypeString:
		result.Kind = KindString
	case openapi3.TypeInteger:
		result.Kind = KindInteger
	case openapi3.TypeNumber:
		result.Kind = KindNumber
	case openapi3.TypeBoolean:
		result.Kind = KindBoolean
	case openapi3.TypeArray:
		if schema.Items == nil {
			return Type{}, fmt.Errorf("array items are required")
		}
		items, err := buildType(schema.Items, classes)
		if err != nil {
			return Type{}, fmt.Errorf("array items: %w", err)
		}
		result.Kind = KindArray
		result.Items = &items
	case openapi3.TypeObject:
		if !isMapSchema(schema) {
			return Type{}, fmt.Errorf("anonymous object schemas are not supported")
		}
		items, err := mapValueType(schema, classes)
		if err != nil {
			return Type{}, err
		}
		result.Kind = KindMap
		result.Items = &items
	default:
		return Type{}, fmt.Errorf("unsupported schema type %q", typeName)
	}
	return result, nil
}

func buildUnion(references openapi3.SchemaRefs, classes map[string]Kind) (Type, error) {
	if len(references) == 0 {
		return Type{}, fmt.Errorf("schema union must contain at least one value")
	}

	values := make([]Type, 0, len(references))
	nullable := false
	for _, reference := range references {
		if isNullSchema(reference) {
			nullable = true
			continue
		}
		value, err := buildType(reference, classes)
		if err != nil {
			return Type{}, fmt.Errorf("union value: %w", err)
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return Type{}, fmt.Errorf("schema union must contain a non-null value")
	}
	if len(values) == 1 {
		values[0].Nullable = values[0].Nullable || nullable
		return values[0], nil
	}
	return Type{Kind: KindUnion, Nullable: nullable, Values: values}, nil
}

func isNullSchema(reference *openapi3.SchemaRef) bool {
	return reference != nil && reference.Ref == "" && reference.Value != nil && reference.Value.Type.Is(openapi3.TypeNull)
}

func schemaType(schema *openapi3.Schema) (bool, string, error) {
	types := schema.Type.Slice()
	if len(types) == 1 {
		if types[0] == openapi3.TypeNull {
			return false, "", fmt.Errorf("null schema requires a non-null type")
		}
		return false, types[0], nil
	}
	if len(types) != 2 || !schema.Type.IncludesNull() {
		return false, "", fmt.Errorf("schema must define one type or one type plus null")
	}
	for _, typeName := range types {
		if typeName != openapi3.TypeNull {
			return true, typeName, nil
		}
	}
	return false, "", fmt.Errorf("null schema requires a non-null type")
}

func validateObjectSchema(name string, schema *openapi3.Schema) error {
	if err := validateSchemaKeywords(schema); err != nil {
		return fmt.Errorf("model %q: %w", name, err)
	}
	if !schema.Type.Is(openapi3.TypeObject) {
		return fmt.Errorf("model %q must be an object", name)
	}
	if isMapSchema(schema) {
		return fmt.Errorf("model %q must not mix properties with additional properties", name)
	}
	return nil
}

func validateSchemaKeywords(schema *openapi3.Schema) error {
	if schema == nil {
		return fmt.Errorf("schema is required")
	}
	if len(schema.AllOf) != 0 || schema.Not != nil {
		return fmt.Errorf("schema allOf and not are not supported")
	}
	if schema.Always != nil || schema.Contains != nil || len(schema.PrefixItems) != 0 || len(schema.PatternProperties) != 0 || schema.If != nil || schema.Then != nil || schema.Else != nil {
		return fmt.Errorf("advanced JSON Schema keywords are not supported")
	}
	return nil
}

func isMapSchema(schema *openapi3.Schema) bool {
	if schema == nil || !schema.Type.Is(openapi3.TypeObject) || len(schema.Properties) != 0 {
		return false
	}
	return schema.AdditionalProperties.Schema != nil || (schema.AdditionalProperties.Has != nil && *schema.AdditionalProperties.Has)
}

func mapValueType(schema *openapi3.Schema, classes map[string]Kind) (Type, error) {
	if err := validateMapKeySchema(schema.PropertyNames); err != nil {
		return Type{}, err
	}
	if schema.AdditionalProperties.Schema != nil {
		return buildType(schema.AdditionalProperties.Schema, classes)
	}
	if schema.AdditionalProperties.Has != nil && *schema.AdditionalProperties.Has {
		return Type{Kind: KindAny}, nil
	}
	return Type{}, fmt.Errorf("object map values are required")
}

func validateMapKeySchema(reference *openapi3.SchemaRef) error {
	if reference == nil {
		return nil
	}
	if reference.Ref != "" || reference.Value == nil {
		return fmt.Errorf("map property names must be defined locally")
	}
	schema := reference.Value
	if len(schema.AllOf) != 0 || len(schema.AnyOf) != 0 || len(schema.OneOf) != 0 || schema.Not != nil || schema.Always != nil || schema.Contains != nil || len(schema.PrefixItems) != 0 || len(schema.PatternProperties) != 0 || schema.PropertyNames != nil || schema.If != nil || schema.Then != nil || schema.Else != nil {
		return fmt.Errorf("map property name schema is not supported")
	}
	if schema.Type != nil && len(schema.Type.Slice()) != 0 && !schema.Type.Is(openapi3.TypeString) {
		return fmt.Errorf("map property names must be strings")
	}
	return nil
}

func componentSchema(name string, reference *openapi3.SchemaRef) (*openapi3.Schema, error) {
	if reference == nil || reference.Value == nil || reference.Ref != "" {
		return nil, fmt.Errorf("component schema %q must be defined locally", name)
	}
	return reference.Value, nil
}

func localSchemaReferenceName(reference string) (string, error) {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(reference, prefix) {
		return "", fmt.Errorf("external schema reference %q is not supported", reference)
	}
	name := strings.TrimPrefix(reference, prefix)
	if name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid component schema reference %q", reference)
	}
	return name, nil
}

func isStringEnum(schema *openapi3.Schema) bool {
	return schema.Type.Is(openapi3.TypeString) && len(schema.Enum) != 0
}

func sortedSchemaNames(schemas openapi3.Schemas) []string {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
