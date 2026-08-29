package contract

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func buildOperations(specification *openapi3.T, classes map[string]Kind) ([]Operation, error) {
	if specification.Paths == nil {
		return nil, fmt.Errorf("OpenAPI paths are required")
	}

	seenIDs := make(map[string]struct{})
	var operations []Operation
	for _, path := range specification.Paths.Keys() {
		item := specification.Paths.Value(path)
		if item == nil || item.Ref != "" {
			return nil, fmt.Errorf("path %q must be defined locally", path)
		}
		if len(item.Parameters) != 0 {
			return nil, fmt.Errorf("path %q path-level parameters are not supported", path)
		}
		if len(item.Servers) != 0 {
			return nil, fmt.Errorf("path %q server overrides are not supported", path)
		}

		methods := item.Operations()
		methodNames := make([]string, 0, len(methods))
		for method := range methods {
			methodNames = append(methodNames, method)
		}
		sort.Strings(methodNames)
		for _, method := range methodNames {
			if method != http.MethodGet && method != http.MethodPost {
				return nil, fmt.Errorf("path %q method %s is not supported", path, method)
			}
			operation, err := buildOperation(methods[method], method, path, classes)
			if err != nil {
				return nil, err
			}
			if _, exists := seenIDs[operation.ID]; exists {
				return nil, fmt.Errorf("operationId %q is duplicated", operation.ID)
			}
			seenIDs[operation.ID] = struct{}{}
			operations = append(operations, operation)
		}
	}

	sort.Slice(operations, func(left, right int) bool {
		if operations[left].Path != operations[right].Path {
			return operations[left].Path < operations[right].Path
		}
		return operations[left].Method < operations[right].Method
	})
	return operations, nil
}

func buildOperation(source *openapi3.Operation, method string, path string, classes map[string]Kind) (Operation, error) {
	if source == nil {
		return Operation{}, fmt.Errorf("path %q method %s is required", path, method)
	}
	if strings.TrimSpace(source.OperationID) == "" {
		return Operation{}, fmt.Errorf("path %q method %s requires operationId", path, method)
	}
	if source.RequestBody != nil {
		return Operation{}, fmt.Errorf("operation %q request bodies are not supported", source.OperationID)
	}
	if len(source.Callbacks) != 0 {
		return Operation{}, fmt.Errorf("operation %q callbacks are not supported", source.OperationID)
	}
	if source.Servers != nil && len(*source.Servers) != 0 {
		return Operation{}, fmt.Errorf("operation %q server overrides are not supported", source.OperationID)
	}

	scopes, err := buildScopes(source)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %q: %w", source.OperationID, err)
	}
	parameters, err := buildParameters(source.Parameters, classes)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %q: %w", source.OperationID, err)
	}
	response, err := buildResponse(source.Responses, classes)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %q: %w", source.OperationID, err)
	}

	return Operation{ID: source.OperationID, Method: method, Path: path, Scopes: scopes, Parameters: parameters, Response: response}, nil
}

func buildScopes(operation *openapi3.Operation) ([]string, error) {
	if operation.Security == nil || len(*operation.Security) != 1 {
		return nil, fmt.Errorf("must define exactly one ApiKey security requirement")
	}
	requirement := (*operation.Security)[0]
	if len(requirement) != 1 {
		return nil, fmt.Errorf("must define exactly one ApiKey security requirement")
	}
	if _, exists := requirement["ApiKey"]; !exists {
		return nil, fmt.Errorf("must define ApiKey security")
	}

	rawScopes, exists := operation.Extensions["x-hostero-required-scopes"]
	if !exists {
		return nil, fmt.Errorf("x-hostero-required-scopes is required")
	}
	values, ok := rawScopes.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("x-hostero-required-scopes must be a non-empty string list")
	}

	scopes := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		scope, ok := value.(string)
		if !ok || strings.TrimSpace(scope) == "" {
			return nil, fmt.Errorf("x-hostero-required-scopes must be a non-empty string list")
		}
		if _, exists := seen[scope]; exists {
			return nil, fmt.Errorf("x-hostero-required-scopes must not contain duplicates")
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	return scopes, nil
}

func buildParameters(references openapi3.Parameters, classes map[string]Kind) ([]Parameter, error) {
	parameters := make([]Parameter, 0, len(references))
	for _, reference := range references {
		if reference == nil || reference.Ref != "" || reference.Value == nil {
			return nil, fmt.Errorf("parameter references are not supported")
		}
		parameter := reference.Value
		if parameter.Content != nil {
			return nil, fmt.Errorf("parameter %q content bodies are not supported", parameter.Name)
		}
		if parameter.Schema == nil {
			return nil, fmt.Errorf("parameter %q schema is required", parameter.Name)
		}

		location, err := parameterLocation(parameter.In)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", parameter.Name, err)
		}
		if location == ParameterPath && !parameter.Required {
			return nil, fmt.Errorf("path parameter %q must be required", parameter.Name)
		}
		typeValue, err := buildType(parameter.Schema, classes)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", parameter.Name, err)
		}
		defaultValue, err := parameterDefault(parameter.Schema, typeValue)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", parameter.Name, err)
		}

		parameters = append(parameters, Parameter{Name: parameter.Name, Location: location, Required: parameter.Required, Default: defaultValue, Type: typeValue})
	}

	sort.Slice(parameters, func(left, right int) bool {
		if parameters[left].Location != parameters[right].Location {
			return parameters[left].Location < parameters[right].Location
		}
		return parameters[left].Name < parameters[right].Name
	})
	return parameters, nil
}

func parameterLocation(location string) (ParameterLocation, error) {
	switch location {
	case openapi3.ParameterInPath:
		return ParameterPath, nil
	case openapi3.ParameterInQuery:
		return ParameterQuery, nil
	default:
		return "", fmt.Errorf("parameter location %q is not supported", location)
	}
}

func parameterDefault(reference *openapi3.SchemaRef, typeValue Type) (any, error) {
	if reference.Ref != "" || reference.Value == nil || reference.Value.Default == nil {
		return nil, nil
	}
	value := reference.Value.Default
	switch typeValue.Kind {
	case KindString:
		if _, ok := value.(string); !ok {
			return nil, fmt.Errorf("default must be a string")
		}
	case KindInteger, KindNumber:
		if _, ok := value.(float64); !ok {
			return nil, fmt.Errorf("default must be a number")
		}
	case KindBoolean:
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("default must be a boolean")
		}
	default:
		return nil, fmt.Errorf("defaults are not supported for %s parameters", typeValue.Kind)
	}
	return value, nil
}

func buildResponse(responses *openapi3.Responses, classes map[string]Kind) (Response, error) {
	if responses == nil {
		return Response{}, fmt.Errorf("responses are required")
	}

	var status string
	for _, code := range responses.Keys() {
		parsed, err := strconv.Atoi(code)
		if err != nil || parsed < 200 || parsed >= 300 {
			continue
		}
		if status != "" {
			return Response{}, fmt.Errorf("exactly one 2xx response is required")
		}
		status = code
	}
	if status == "" {
		return Response{}, fmt.Errorf("exactly one 2xx response is required")
	}

	code, _ := strconv.Atoi(status)
	reference := responses.Value(status)
	if reference == nil || reference.Ref != "" || reference.Value == nil {
		return Response{}, fmt.Errorf("response %s must be defined locally", status)
	}
	response := reference.Value
	if len(response.Headers) != 0 || len(response.Links) != 0 {
		return Response{}, fmt.Errorf("response %s headers and links are not supported", status)
	}
	if code == http.StatusNoContent {
		if len(response.Content) != 0 {
			return Response{}, fmt.Errorf("response 204 must not define content")
		}
		return Response{Status: code}, nil
	}
	if len(response.Content) != 1 || response.Content["application/json"] == nil {
		return Response{}, fmt.Errorf("response %s must define only application/json content", status)
	}
	mediaType := response.Content["application/json"]
	if mediaType.Schema == nil {
		return Response{}, fmt.Errorf("response %s JSON schema is required", status)
	}
	typeValue, err := buildType(mediaType.Schema, classes)
	if err != nil {
		return Response{}, fmt.Errorf("response %s: %w", status, err)
	}
	return Response{Status: code, Type: &typeValue}, nil
}
