package contract

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const clientResourceExtension = "x-hostero-client-resource"

var (
	resourceKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	clientGroupPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

func buildResources(components *openapi3.Components, models []Model, operations []Operation) ([]Resource, error) {
	if components == nil {
		return nil, fmt.Errorf("OpenAPI components are required")
	}
	modelsByName := make(map[string]Model, len(models))
	for _, model := range models {
		modelsByName[model.Name] = model
	}

	resources := make([]Resource, 0)
	byKind := make(map[string]Resource)
	for _, name := range sortedSchemaNames(components.Schemas) {
		schema, err := componentSchema(name, components.Schemas[name])
		if err != nil {
			return nil, err
		}
		raw, found := schema.Extensions[clientResourceExtension]
		if !found {
			continue
		}
		model, found := modelsByName[name]
		if !found {
			return nil, fmt.Errorf("component schema %q: %s requires an object model", name, clientResourceExtension)
		}
		resource, err := parseResource(name, raw)
		if err != nil {
			return nil, err
		}
		if err := validateResourceIDField(resource, model); err != nil {
			return nil, err
		}
		if existing, found := byKind[resource.Kind]; found {
			if existing.IDField != resource.IDField || existing.PathParameter != resource.PathParameter || strings.Join(existing.Group, ".") != strings.Join(resource.Group, ".") {
				return nil, fmt.Errorf("resource kind %q has conflicting declarations on %q and %q", resource.Kind, existing.Model, resource.Model)
			}
		} else {
			byKind[resource.Kind] = resource
		}
		resources = append(resources, resource)
	}

	for _, resource := range resources {
		if !hasBindableOperation(resource, operations) {
			return nil, fmt.Errorf("resource %q has no matching resource-scoped operation with path parameter %q", resource.Model, resource.PathParameter)
		}
	}
	return resources, nil
}

func parseResource(model string, raw any) (Resource, error) {
	mapping, ok := raw.(map[string]any)
	if !ok {
		return Resource{}, fmt.Errorf("component schema %q: %s must be an object", model, clientResourceExtension)
	}
	if len(mapping) != 4 {
		return Resource{}, fmt.Errorf("component schema %q: %s must define exactly kind, id_field, group, and path_parameter", model, clientResourceExtension)
	}
	kind, ok := mapping["kind"].(string)
	if !ok || !resourceKindPattern.MatchString(kind) {
		return Resource{}, fmt.Errorf("component schema %q: %s kind must be a lower snake-case identifier", model, clientResourceExtension)
	}
	idField, ok := mapping["id_field"].(string)
	if !ok || strings.TrimSpace(idField) == "" {
		return Resource{}, fmt.Errorf("component schema %q: %s id_field must be a non-empty string", model, clientResourceExtension)
	}
	pathParameter, ok := mapping["path_parameter"].(string)
	if !ok || strings.TrimSpace(pathParameter) == "" {
		return Resource{}, fmt.Errorf("component schema %q: %s path_parameter must be a non-empty string", model, clientResourceExtension)
	}
	group, err := parseResourceGroup(model, mapping["group"])
	if err != nil {
		return Resource{}, err
	}
	return Resource{
		Model:         model,
		Kind:          kind,
		IDField:       idField,
		Group:         group,
		PathParameter: pathParameter,
	}, nil
}

func parseResourceGroup(model string, raw any) ([]string, error) {
	values, ok := raw.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("component schema %q: %s group must be a non-empty list of lower-kebab identifiers", model, clientResourceExtension)
	}
	group := make([]string, 0, len(values))
	for _, value := range values {
		name, ok := value.(string)
		if !ok || !clientGroupPattern.MatchString(name) {
			return nil, fmt.Errorf("component schema %q: %s group must contain lower-kebab identifiers", model, clientResourceExtension)
		}
		group = append(group, name)
	}
	return group, nil
}

func validateResourceIDField(resource Resource, model Model) error {
	for _, field := range model.Fields {
		if field.Name != resource.IDField {
			continue
		}
		if !field.Required || field.Type.Kind != KindString || field.Type.Nullable {
			return fmt.Errorf("resource %q id_field %q must be a required non-null string model field", resource.Model, resource.IDField)
		}
		return nil
	}
	return fmt.Errorf("resource %q id_field %q is not a model field", resource.Model, resource.IDField)
}

func hasBindableOperation(resource Resource, operations []Operation) bool {
	for _, operation := range operations {
		if !hasPrefix(operation.ClientMetadata.Group, resource.Group) || !containsString(operation.TargetKinds, resource.Kind) {
			continue
		}
		for _, parameter := range operation.Parameters {
			if parameter.Location == ParameterPath && parameter.Required && parameter.Name == resource.PathParameter {
				return true
			}
		}
	}
	return false
}

func hasPrefix(values []string, prefix []string) bool {
	if len(values) < len(prefix) {
		return false
	}
	for index, value := range prefix {
		if values[index] != value {
			return false
		}
	}
	return true
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
