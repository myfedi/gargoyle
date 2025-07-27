package utils

import (
	"bytes"
	"text/template"
)

type FormatParams map[string]interface{}

// NamedFormat formats a string using a template with named parameters.
// It takes a format string and a map of parameters, and returns the formatted string.
// Example usage:
//
//	format := "Hello, {{.name}}! You have {{.count}} new messages."
//	params := FormatParams{"name": "Alice", "count": 5}
//	result, err := NamedFormat(format, params)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(result) // Output: Hello, Alice! You have 5 new messages
func NamedFormat(format string, params FormatParams) (string, error) {
	var buf bytes.Buffer

	tmpl, err := template.New("fmtstr").Parse(format)
	if err != nil {
		return "", err
	}

	err = tmpl.Execute(&buf, params)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
