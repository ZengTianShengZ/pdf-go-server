package service

import (
	"bytes"
	"fmt"
	"os"
)

const reportDataPrefix = "window.__REPORT_DATA__"

// InjectReportData reads the HTML template and replaces only the object literal
// after window.__REPORT_DATA__ =, keeping the assignment statement.
func InjectReportData(templatePath string, jsonObject []byte) ([]byte, error) {
	raw, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}

	start, end, err := findReportDataObjectSpan(raw)
	if err != nil {
		return nil, err
	}

	var out bytes.Buffer
	out.Grow(len(raw) - (end - start) + len(jsonObject))
	out.Write(raw[:start])
	out.Write(jsonObject)
	out.Write(raw[end:])
	return out.Bytes(), nil
}

// findReportDataObjectSpan returns the byte range [start, end) of the `{...}`
// object literal assigned to window.__REPORT_DATA__.
func findReportDataObjectSpan(raw []byte) (int, int, error) {
	idx := bytes.Index(raw, []byte(reportDataPrefix))
	if idx < 0 {
		return 0, 0, fmt.Errorf("template missing window.__REPORT_DATA__ assignment")
	}

	rest := raw[idx+len(reportDataPrefix):]
	eq := bytes.IndexByte(rest, '=')
	if eq < 0 {
		return 0, 0, fmt.Errorf("template missing window.__REPORT_DATA__ assignment")
	}
	pos := idx + len(reportDataPrefix) + eq + 1

	for pos < len(raw) && (raw[pos] == ' ' || raw[pos] == '\t' || raw[pos] == '\n' || raw[pos] == '\r') {
		pos++
	}
	if pos >= len(raw) || raw[pos] != '{' {
		return 0, 0, fmt.Errorf("template missing window.__REPORT_DATA__ object literal")
	}

	end, err := scanJSONObjectEnd(raw, pos)
	if err != nil {
		return 0, 0, err
	}
	return pos, end, nil
}

func scanJSONObjectEnd(raw []byte, start int) (int, error) {
	depth := 0
	inString := false
	escape := false

	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			switch c {
			case '\\':
				escape = true
			case '"':
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
			if depth < 0 {
				return 0, fmt.Errorf("unbalanced braces in __REPORT_DATA__")
			}
		}
	}
	return 0, fmt.Errorf("unclosed __REPORT_DATA__ object")
}

// EnsureJSONObjectRoot returns an error if data is not a JSON object root.
func EnsureJSONObjectRoot(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("body must be a JSON object")
	}
	return nil
}
