package apispec

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// gRPC .proto parsing. Produces APIContract / Operation records using
// the same shape as ParseOpenAPI / ParseGraphQL — gRPC services are
// the third major cross-stack contract format, and consuming them
// through the same Operation type lets impact analysis treat REST,
// GraphQL, and gRPC equivalently for field-level narrowing.
//
// Parsing handles proto3 + proto2 syntax. The targets are `service`
// declarations and their `rpc` methods; `message` definitions are
// resolved to populate FieldsRead (the RPC's response fields) and
// FieldsWrite (the request fields). Streaming RPCs (client-stream,
// server-stream, bidi) are recognized and their Method records carry
// the streaming-flavor suffix.

// ParseProtoFile reads a .proto file and emits an APIContract.
func ParseProtoFile(path string) (*APIContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("apispec: read proto %s: %w", path, err)
	}
	c := ParseProto(string(data))
	c.Path = path
	return c, nil
}

// ParseProto extracts services + RPC methods from a .proto source.
func ParseProto(src string) *APIContract {
	c := &APIContract{Kind: ContractGRPC}

	clean := stripProtoComments(src)

	// Determine syntax version from the syntax declaration.
	if m := protoSyntaxRE.FindStringSubmatch(clean); len(m) == 2 {
		c.Version = m[1]
	}

	// Extract messages first so we can populate FieldsRead/FieldsWrite
	// on the RPCs.
	messages := parseProtoMessages(clean)

	// Walk service blocks.
	for _, svc := range findBlockBodies(clean, "service") {
		serviceName := svc.name
		for _, m := range protoRPCRE.FindAllStringSubmatch(svc.body, -1) {
			method := m[1]
			clientStream := strings.TrimSpace(m[2]) == "stream"
			reqType := m[3]
			serverStream := strings.TrimSpace(m[4]) == "stream"
			respType := m[5]

			op := Operation{
				Method:      protoRPCMethodLabel(clientStream, serverStream),
				Path:        "/" + serviceName + "/" + method,
				OperationID: serviceName + "." + method,
				Summary:     "",
			}
			if fields, ok := messages[stripProtoQualifier(respType)]; ok {
				op.FieldsRead = append(op.FieldsRead, fields...)
			}
			if fields, ok := messages[stripProtoQualifier(reqType)]; ok {
				op.FieldsWrite = append(op.FieldsWrite, fields...)
			}
			c.Operations = append(c.Operations, op)
		}
	}

	return c
}

// ContractGRPC identifies gRPC service contracts. Declared here rather
// than in apispec.go's const block so the openapi/graphql consts stay
// adjacent in that file's reading order.
const ContractGRPC ContractKind = "grpc"

var (
	protoSyntaxRE = regexp.MustCompile(`(?m)^\s*syntax\s*=\s*"([^"]+)"`)
	// rpc Method(stream Request) returns (stream Response) { ... }
	// Groups: 1 method, 2 "stream" (or empty), 3 request type, 4 "stream" (or empty), 5 response type.
	protoRPCRE = regexp.MustCompile(`(?is)\brpc\s+(\w+)\s*\(\s*(stream\s+)?([\w\.]+)\s*\)\s*returns\s*\(\s*(stream\s+)?([\w\.]+)\s*\)`)
)

// findBlockBodies finds `<keyword> Name { ... }` blocks and returns
// the name + body for each, respecting nested braces.
type protoBlock struct {
	name string
	body string
}

func findBlockBodies(src, keyword string) []protoBlock {
	var out []protoBlock
	pattern := regexp.MustCompile(`\b` + keyword + `\s+(\w+)\s*\{`)
	for _, m := range pattern.FindAllStringSubmatchIndex(src, -1) {
		// m[2:4] are the indices of the name capture group; m[1] is
		// the end of the entire match (just past `{`).
		name := src[m[2]:m[3]]
		bodyStart := m[1]
		// Walk braces to find the matching close.
		depth := 1
		i := bodyStart
		for i < len(src) && depth > 0 {
			switch src[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			i++
		}
		if depth == 0 {
			out = append(out, protoBlock{name: name, body: src[bodyStart : i-1]})
		}
	}
	return out
}

// parseProtoMessages returns a map of MessageName → list of field
// names. The implementation deliberately handles only top-level
// fields; nested messages and oneofs are flattened to their direct
// fields. Repeated / map fields are recorded by name once.
func parseProtoMessages(src string) map[string][]string {
	out := map[string][]string{}
	for _, blk := range findBlockBodies(src, "message") {
		out[blk.name] = parseMessageFields(blk.body)
	}
	return out
}

var (
	// `int32 field_name = 1;` or `repeated string foo = 2;` or `MyMsg sub = 3;`
	// Skips reserved / option / enum / message lines; matches `<type> <name> = <tag>;`
	protoFieldRE = regexp.MustCompile(`(?m)^\s*(?:repeated\s+|optional\s+|required\s+)?[\w\.<>,\s]+\s+(\w+)\s*=\s*\d+\s*(?:\[[^\]]*\])?\s*;`)
)

// parseMessageFields extracts field names from a message body,
// skipping nested message / enum / oneof / reserved blocks.
func parseMessageFields(body string) []string {
	// Strip nested blocks (oneof, enum, nested message). They have
	// their own structure, and their fields don't belong to the parent.
	clean := removeNestedBlocks(body)

	var fields []string
	seen := map[string]bool{}
	for _, m := range protoFieldRE.FindAllStringSubmatch(clean, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		if isProtoKeyword(name) {
			continue
		}
		seen[name] = true
		fields = append(fields, name)
	}
	sortStrings(fields)
	return fields
}

// removeNestedBlocks strips `oneof X { ... }`, `enum X { ... }`,
// `message X { ... }` blocks from a message body so the field
// extractor sees only the direct fields.
//
// Implementation: scan for the keywords at any indentation; when
// found, walk braces to the matching close and elide.
func removeNestedBlocks(body string) string {
	var out strings.Builder
	i := 0
	for i < len(body) {
		// Look for nested-block keywords at this position.
		kw := matchNestedKeyword(body, i)
		if kw == "" {
			out.WriteByte(body[i])
			i++
			continue
		}
		// Find opening brace.
		brace := strings.Index(body[i:], "{")
		if brace < 0 {
			// No brace — write the rest and stop.
			out.WriteString(body[i:])
			return out.String()
		}
		// Walk to matching close.
		j := i + brace + 1
		depth := 1
		for j < len(body) && depth > 0 {
			switch body[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
			j++
		}
		i = j
	}
	return out.String()
}

func matchNestedKeyword(body string, i int) string {
	// Only match at start-of-line or after whitespace.
	if i > 0 {
		prev := body[i-1]
		if prev != ' ' && prev != '\t' && prev != '\n' && prev != '\r' && prev != '{' {
			return ""
		}
	}
	for _, kw := range []string{"oneof", "enum", "message"} {
		if i+len(kw) > len(body) {
			continue
		}
		if body[i:i+len(kw)] != kw {
			continue
		}
		// Next char must be whitespace.
		next := byte(0)
		if i+len(kw) < len(body) {
			next = body[i+len(kw)]
		}
		if next == ' ' || next == '\t' {
			return kw
		}
	}
	return ""
}

func isProtoKeyword(name string) bool {
	switch name {
	case "reserved", "option", "extensions", "map":
		return true
	}
	return false
}

// stripProtoQualifier removes the package prefix from a type
// reference so we can look it up in the messages map.
// `mypkg.MyMessage` → `MyMessage`, `google.protobuf.Empty` → `Empty`.
func stripProtoQualifier(t string) string {
	if i := strings.LastIndex(t, "."); i >= 0 {
		return t[i+1:]
	}
	return t
}

// protoRPCMethodLabel produces the RPC method label including
// streaming flavor when applicable.
func protoRPCMethodLabel(clientStream, serverStream bool) string {
	switch {
	case clientStream && serverStream:
		return "RPC-BIDI"
	case clientStream:
		return "RPC-CLIENT-STREAM"
	case serverStream:
		return "RPC-SERVER-STREAM"
	default:
		return "RPC"
	}
}

// stripProtoComments removes `//` line comments and `/* */` block
// comments from a proto source.
func stripProtoComments(src string) string {
	var b strings.Builder
	i := 0
	for i < len(src) {
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '/' {
			for i < len(src) && src[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(src) && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			if i+1 < len(src) {
				i += 2
			}
			continue
		}
		b.WriteByte(src[i])
		i++
	}
	return b.String()
}
