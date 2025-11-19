// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hotspot // import "go.opentelemetry.io/ebpf-profiler/interpreter/hotspot"

import (
	"io"
	"strings"
)

// javaBaseTypes maps a basic type signature character to the full type name
var javaBaseTypes = map[byte]string{
	'B': "byte",
	'C': "char",
	'D': "double",
	'F': "float",
	'I': "int",
	'J': "long",
	'S': "short",
	'V': "void",
	'Z': "boolean",
}

// demangleJavaTypeSignature demangles a JavaTypeSignature
func demangleJavaTypeSignature(signature string, sb io.StringWriter) string {
	var i, numArr int
	for i = 0; i < len(signature) && signature[i] == '['; i++ {
		numArr++
	}
	if i >= len(signature) {
		return ""
	}

	typeChar := signature[i]
	i++

	if typeChar == 'L' {
		end := strings.IndexByte(signature, ';')
		if end < 0 {
			return ""
		}
		_, _ = sb.WriteString(strings.ReplaceAll(signature[i:end], "/", "."))
		i = end + 1
	} else if typeStr, ok := javaBaseTypes[typeChar]; ok {
		_, _ = sb.WriteString(typeStr)
	}

	for numArr > 0 {
		_, _ = sb.WriteString("[]")
		numArr--
	}

	if len(signature) > i {
		return signature[i:]
	}
	return ""
}

// demangleJavaSignature demangles a JavaTypeSignature
func demangleJavaMethod(klass, method, signature string) string {
	var sb strings.Builder

	// Name format is specified in
	//  - Java Virtual Machine Specification (JVMS)
	//    https://docs.oracle.com/javase/specs/jvms/se14/jvms14.pdf
	//  - Java Language Specification (JLS)
	//    https://docs.oracle.com/javase/specs/jls/se13/jls13.pdf
	//
	// see: JVMS §4.2 (name encoding), §4.3 (signature descriptors)
	//      JLS §13.1 (name encoding)
	//
	// Scala has additional internal transformations which are not
	// well defined, and have changed between Scala versions.

	// Signature looks like "(argumentsSignatures)returnValueSignature"
	// Check for the parenthesis first.
	end := strings.IndexByte(signature, ')')
	if end < 0 || signature[0] != '(' {
		return ""
	}

	left := demangleJavaTypeSignature(signature[end+1:], &sb)
	if left != "" {
		return ""
	}
	sb.WriteRune(' ')
	sb.WriteString(strings.ReplaceAll(klass, "/", "."))
	sb.WriteRune('.')
	sb.WriteString(method)
	sb.WriteRune('(')
	left = signature[1:end]
	for left != "" {
		left = demangleJavaTypeSignature(left, &sb)
		if left == "" {
			break
		}
		sb.WriteString(", ")
	}
	sb.WriteRune(')')

	return sb.String()
}
