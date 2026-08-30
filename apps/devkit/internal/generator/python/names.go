package python

import (
	"fmt"
	"strings"
	"unicode"
)

var keywords = map[string]struct{}{
	"False": {}, "None": {}, "True": {}, "and": {}, "as": {}, "assert": {}, "async": {}, "await": {},
	"break": {}, "class": {}, "continue": {}, "def": {}, "del": {}, "elif": {}, "else": {}, "except": {},
	"finally": {}, "for": {}, "from": {}, "global": {}, "if": {}, "import": {}, "in": {}, "is": {},
	"lambda": {}, "nonlocal": {}, "not": {}, "or": {}, "pass": {}, "raise": {}, "return": {}, "try": {},
	"while": {}, "with": {}, "yield": {},
}

func className(value string) (string, error) {
	words := identifierWords(value)
	if len(words) == 0 {
		return "", fmt.Errorf("cannot derive a Python class name from %q", value)
	}

	var builder strings.Builder
	for _, word := range words {
		builder.WriteString(titleWord(word))
	}
	name := builder.String()
	if first := []rune(name)[0]; unicode.IsDigit(first) {
		name = "Model" + name
	}
	if _, isKeyword := keywords[name]; isKeyword {
		name += "Model"
	}
	return name, nil
}

func fieldName(value string) (string, error) {
	words := identifierWords(value)
	if len(words) == 0 {
		return "", fmt.Errorf("cannot derive a Python field name from %q", value)
	}

	name := strings.Join(words, "_")
	if first := []rune(name)[0]; unicode.IsDigit(first) {
		name = "_" + name
	}
	if _, isKeyword := keywords[name]; isKeyword {
		name += "_"
	}
	return name, nil
}

func methodName(value string) (string, error) {
	words := identifierWords(value)
	if len(words) == 0 {
		return "", fmt.Errorf("cannot derive a Python method name from %q", value)
	}

	name := strings.Join(words, "_")
	if first := []rune(name)[0]; unicode.IsDigit(first) {
		name = "_" + name
	}
	if _, isKeyword := keywords[name]; isKeyword {
		name += "_"
	}
	return name, nil
}

func serviceClassName(group []string) (string, error) {
	if len(group) == 0 {
		return "ServicesMixin", nil
	}
	var builder strings.Builder
	for _, segment := range group {
		words := identifierWords(segment)
		if len(words) == 0 {
			return "", fmt.Errorf("cannot derive service class name from segment %q", segment)
		}
		for _, word := range words {
			builder.WriteString(titleWord(word))
		}
	}
	builder.WriteString("Service")
	return builder.String(), nil
}

func groupFieldName(segment string) (string, error) {
	return fieldName(segment)
}

func enumMemberName(value string) (string, error) {
	words := identifierWords(value)
	if len(words) == 0 {
		return "", fmt.Errorf("cannot derive a Python enum member name from %q", value)
	}

	name := strings.ToUpper(strings.Join(words, "_"))
	if first := []rune(name)[0]; unicode.IsDigit(first) {
		name = "VALUE_" + name
	}
	return name, nil
}

func identifierWords(value string) []string {
	runes := []rune(value)
	words := make([]string, 0, 2)
	current := make([]rune, 0, len(runes))

	flush := func() {
		if len(current) == 0 {
			return
		}
		words = append(words, strings.ToLower(string(current)))
		current = current[:0]
	}

	for index, runeValue := range runes {
		if !unicode.IsLetter(runeValue) && !unicode.IsDigit(runeValue) {
			flush()
			continue
		}

		if len(current) != 0 {
			previous := current[len(current)-1]
			nextIsLower := index+1 < len(runes) && unicode.IsLower(runes[index+1])
			if unicode.IsUpper(runeValue) && (unicode.IsLower(previous) || unicode.IsDigit(previous)) {
				flush()
			} else if unicode.IsUpper(previous) && unicode.IsUpper(runeValue) && nextIsLower && len(current) > 1 {
				flush()
			}
		}
		current = append(current, runeValue)
	}
	flush()
	return words
}

func titleWord(word string) string {
	runes := []rune(word)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
