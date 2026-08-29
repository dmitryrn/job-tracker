package services

import (
	"strings"
	"unicode"
)

func canonicalConcept(concept string) string {
	concept = strings.ToLower(strings.TrimSpace(concept))
	switch concept {
	case "golang":
		return "go"
	case "type script", "type-script":
		return "typescript"
	case "c++":
		return "cpp"
	case "c#":
		return "csharp"
	}

	var normalized strings.Builder
	previousSeparator := false
	for _, character := range concept {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			normalized.WriteRune(character)
			previousSeparator = false
			continue
		}
		if !previousSeparator && normalized.Len() > 0 {
			normalized.WriteByte('_')
			previousSeparator = true
		}
	}
	return strings.Trim(normalized.String(), "_")
}
