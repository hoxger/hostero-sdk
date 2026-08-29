package contract

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

const (
	requiredPermissionsExtension = "x-hostero-required-permissions"
	targetKindsExtension         = "x-hostero-target-kind"
	clientMetadataExtension      = "x-hostero-client"
)

func buildOperations(paths *openapi3.Paths, classes map[string]Kind) ([]Operation, error) {
	if paths == nil {
		return nil, fmt.Errorf("OpenAPI paths are required")
	}

	operations := make([]Operation, 0)
	seenIDs := make(map[string]string)
	seenClientTargets := make(map[string]string)
	for _, path := range sortedPathNames(paths) {
		pathItem := paths.Value(path)
		if pathItem == nil || pathItem.Ref != "" {
			return nil, fmt.Errorf("path %q must be defined locally", path)
		}
		for method, sourceOperation := range pathOperations(pathItem) {
			if sourceOperation == nil {
				continue
			}
			operation, err := buildOperation(path, method, pathItem.Parameters, sourceOperation, classes)
			if err != nil {
				return nil, err
			}
			if existing, found := seenIDs[operation.ID]; found {
				return nil, fmt.Errorf("operation %q duplicates %s", operation.ID, existing)
			}
			seenIDs[operation.ID] = fmt.Sprintf("%s %s", operation.Method, operation.Path)

			clientKey := fmt.Sprintf("%s:%s", strings.Join(operation.ClientMetadata.Group, "."), operation.ClientMetadata.Method)
			if existing, found := seenClientTargets[clientKey]; found {
				return nil, fmt.Errorf("client operation %q declared on %s duplicates %s", clientKey, operation.ID, existing)
			}
			seenClientTargets[clientKey] = operation.ID

			operations = append(operations, operation)
		}
	}
	sort.Slice(operations, func(left int, right int) bool {
		if operations[left].Path != operations[right].Path {
			return operations[left].Path < operations[right].Path
		}
		return operations[left].Method < operations[right].Method
	})
	return operations, nil
}

func buildOperation(path string, method string, pathParameters openapi3.Parameters, source *openapi3.Operation, classes map[string]Kind) (Operation, error) {
	id := strings.TrimSpace(source.OperationID)
	if id == "" {
		return Operation{}, fmt.Errorf("operation %s %s has no operationId", method, path)
	}
	if !requiresAPIKey(source.Security) {
		return Operation{}, fmt.Errorf("operation %s %s must require ApiKey security", method, path)
	}
	permissions, err := stringListExtension(source, requiredPermissionsExtension, true)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %s %s: %w", method, path, err)
	}
	targetKinds, err := stringListExtension(source, targetKindsExtension, false)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %s %s: %w", method, path, err)
	}
	clientMetadata, err := parseClientMetadata(source)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %s %s: %w", method, path, err)
	}
	parameters, err := buildParameters(pathParameters, source.Parameters, classes)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %s %s parameters: %w", method, path, err)
	}
	requestBody, err := buildRequestBody(source.RequestBody, classes)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %s %s request body: %w", method, path, err)
	}
	success, errors, err := buildResponses(source.Responses, classes)
	if err != nil {
		return Operation{}, fmt.Errorf("operation %s %s responses: %w", method, path, err)
	}
	return Operation{
		ID:             id,
		Method:         method,
		Path:           path,
		Tags:           append([]string(nil), source.Tags...),
		Permissions:    permissions,
		TargetKinds:    targetKinds,
		ClientMetadata: clientMetadata,
		Parameters:     parameters,
		RequestBody:    requestBody,
		Success:        success,
		Errors:         errors,
	}, nil
}

func pathOperations(path *openapi3.PathItem) map[string]*openapi3.Operation {
	return map[string]*openapi3.Operation{
		"DELETE": path.Delete,
		"GET":    path.Get,
		"HEAD":   path.Head,
		"PATCH":  path.Patch,
		"POST":   path.Post,
		"PUT":    path.Put,
	}
}

func sortedPathNames(paths *openapi3.Paths) []string {
	names := paths.Keys()
	sort.Strings(names)
	return names
}

func requiresAPIKey(requirements *openapi3.SecurityRequirements) bool {
	if requirements == nil {
		return false
	}
	for _, requirement := range *requirements {
		if _, found := requirement["ApiKey"]; found {
			return true
		}
	}
	return false
}

func stringListExtension(operation *openapi3.Operation, name string, required bool) ([]string, error) {
	value, found := operation.Extensions[name]
	if !found {
		if required {
			return nil, fmt.Errorf("%s is required", name)
		}
		return nil, nil
	}
	values, ok := value.([]any)
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("%s must be a non-empty list of strings", name)
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		item, ok := value.(string)
		if !ok || strings.TrimSpace(item) == "" {
			return nil, fmt.Errorf("%s must be a non-empty list of strings", name)
		}
		if _, found := seen[item]; found {
			return nil, fmt.Errorf("%s contains duplicate %q", name, item)
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result, nil
}

func buildParameters(pathParameters openapi3.Parameters, operationParameters openapi3.Parameters, classes map[string]Kind) ([]Parameter, error) {
	merged := make(map[string]*openapi3.ParameterRef, len(pathParameters)+len(operationParameters))
	for _, parameters := range []openapi3.Parameters{pathParameters, operationParameters} {
		for _, reference := range parameters {
			if reference == nil || reference.Ref != "" || reference.Value == nil {
				return nil, fmt.Errorf("parameters must be defined locally")
			}
			key := reference.Value.In + "\x00" + reference.Value.Name
			merged[key] = reference
		}
	}

	parameters := make([]Parameter, 0, len(merged))
	for _, key := range sortedKeys(merged) {
		source := merged[key].Value
		location, err := parameterLocation(source.In)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", source.Name, err)
		}
		if strings.TrimSpace(source.Name) == "" {
			return nil, fmt.Errorf("parameter name is required")
		}
		if location == ParameterPath && !source.Required {
			return nil, fmt.Errorf("path parameter %q must be required", source.Name)
		}
		if source.Schema == nil || len(source.Content) != 0 {
			return nil, fmt.Errorf("parameter %q must define a schema without content", source.Name)
		}
		typeValue, err := buildType(source.Schema, classes)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", source.Name, err)
		}
		parameters = append(parameters, Parameter{
			Name:     source.Name,
			Location: location,
			Required: source.Required,
			Type:     typeValue,
		})
	}
	sort.Slice(parameters, func(left int, right int) bool {
		if parameters[left].Location != parameters[right].Location {
			return parameters[left].Location < parameters[right].Location
		}
		return parameters[left].Name < parameters[right].Name
	})
	return parameters, nil
}

func parameterLocation(value string) (ParameterLocation, error) {
	switch ParameterLocation(value) {
	case ParameterPath, ParameterQuery, ParameterHeader, ParameterCookie:
		return ParameterLocation(value), nil
	default:
		return "", fmt.Errorf("unsupported location %q", value)
	}
}

func buildRequestBody(reference *openapi3.RequestBodyRef, classes map[string]Kind) (*RequestBody, error) {
	if reference == nil {
		return nil, nil
	}
	if reference.Ref != "" || reference.Value == nil {
		return nil, fmt.Errorf("must be defined locally")
	}
	contentType, schema, err := singleContentSchema(reference.Value.Content)
	if err != nil {
		return nil, err
	}
	typeValue, err := buildType(schema, classes)
	if err != nil {
		return nil, err
	}
	return &RequestBody{Required: reference.Value.Required, ContentType: contentType, Type: typeValue}, nil
}

func buildResponses(responses *openapi3.Responses, classes map[string]Kind) (Response, []Response, error) {
	if responses == nil || responses.Len() == 0 || responses.Default() != nil {
		return Response{}, nil, fmt.Errorf("must define explicit status responses")
	}

	var successes []Response
	var errors []Response
	for _, statusKey := range responses.Keys() {
		status, err := strconv.Atoi(statusKey)
		if err != nil || status < 100 || status > 599 {
			return Response{}, nil, fmt.Errorf("invalid status response %q", statusKey)
		}
		response, err := buildResponse(status, responses.Value(statusKey), classes)
		if err != nil {
			return Response{}, nil, err
		}
		if status >= 200 && status < 400 {
			successes = append(successes, response)
		} else if status >= 400 {
			errors = append(errors, response)
		}
	}
	if len(successes) != 1 {
		return Response{}, nil, fmt.Errorf("must define exactly one 2xx or 3xx success response")
	}
	sort.Slice(errors, func(left int, right int) bool { return errors[left].Status < errors[right].Status })
	return successes[0], errors, nil
}

func buildResponse(status int, reference *openapi3.ResponseRef, classes map[string]Kind) (Response, error) {
	if reference == nil || reference.Ref != "" || reference.Value == nil {
		return Response{}, fmt.Errorf("status %d must be defined locally", status)
	}
	if len(reference.Value.Headers) != 0 {
		return Response{}, fmt.Errorf("status %d response headers are not supported", status)
	}
	if len(reference.Value.Content) == 0 {
		return Response{Status: status}, nil
	}
	contentType, schema, err := singleContentSchema(reference.Value.Content)
	if err != nil {
		return Response{}, fmt.Errorf("status %d: %w", status, err)
	}
	typeValue, err := buildType(schema, classes)
	if err != nil {
		return Response{}, fmt.Errorf("status %d: %w", status, err)
	}
	return Response{Status: status, ContentType: contentType, Type: &typeValue}, nil
}

func singleContentSchema(content openapi3.Content) (string, *openapi3.SchemaRef, error) {
	if len(content) != 1 {
		return "", nil, fmt.Errorf("must define exactly one content type")
	}
	for contentType, mediaType := range content {
		if strings.TrimSpace(contentType) == "" || mediaType == nil || mediaType.Schema == nil {
			return "", nil, fmt.Errorf("must define one content type with a schema")
		}
		return contentType, mediaType.Schema, nil
	}
	return "", nil, fmt.Errorf("must define exactly one content type")
}

func sortedKeys(values map[string]*openapi3.ParameterRef) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parseClientMetadata(source *openapi3.Operation) (ClientMetadata, error) {
	raw, found := source.Extensions[clientMetadataExtension]
	if !found {
		return ClientMetadata{}, fmt.Errorf("%s is required", clientMetadataExtension)
	}
	mapping, ok := raw.(map[string]any)
	if !ok {
		return ClientMetadata{}, fmt.Errorf("%s must be an object", clientMetadataExtension)
	}
	rawGroup, hasGroup := mapping["group"]
	if !hasGroup {
		return ClientMetadata{}, fmt.Errorf("%s missing 'group'", clientMetadataExtension)
	}
	rawGroupList, ok := rawGroup.([]any)
	if !ok || len(rawGroupList) == 0 {
		return ClientMetadata{}, fmt.Errorf("%s 'group' must be a non-empty list of strings", clientMetadataExtension)
	}
	group := make([]string, 0, len(rawGroupList))
	for _, item := range rawGroupList {
		str, ok := item.(string)
		if !ok || strings.TrimSpace(str) == "" {
			return ClientMetadata{}, fmt.Errorf("%s 'group' item must be a non-empty string", clientMetadataExtension)
		}
		group = append(group, str)
	}

	rawMethod, hasMethod := mapping["method"]
	if !hasMethod {
		return ClientMetadata{}, fmt.Errorf("%s missing 'method'", clientMetadataExtension)
	}
	method, ok := rawMethod.(string)
	if !ok || strings.TrimSpace(method) == "" {
		return ClientMetadata{}, fmt.Errorf("%s 'method' must be a non-empty string", clientMetadataExtension)
	}

	return ClientMetadata{
		Group:  group,
		Method: method,
	}, nil
}
