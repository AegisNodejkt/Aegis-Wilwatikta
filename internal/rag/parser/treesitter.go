package parser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type TSParser struct {
	languages map[string]*sitter.Language
	queries   map[string]string
}

type ParseResult struct {
	Nodes     []domain.CodeNode
	Relations []domain.CodeRelation
	Errors    []ParseError
}

type ParseError struct {
	Message string
	Line    int
	Column  int
}

func NewTSParser() *TSParser {
	return &TSParser{
		languages: map[string]*sitter.Language{
			".go":  golang.GetLanguage(),
			".rs":  rust.GetLanguage(),
			".py":  python.GetLanguage(),
			".js":  javascript.GetLanguage(),
			".mjs": javascript.GetLanguage(),
			".cjs": javascript.GetLanguage(),
			".ts":  typescript.GetLanguage(),
			".tsx": tsx.GetLanguage(),
			".mts": typescript.GetLanguage(),
			".cts": typescript.GetLanguage(),
		},
		queries: map[string]string{
			".go": `
(function_declaration name: (identifier) @func.name) @func.def
(method_declaration name: (field_identifier) @method.name) @method.def
(type_declaration (type_spec name: (type_identifier) @struct.name)) @struct.def
(type_declaration (type_spec name: (type_identifier) @interface.name type: (interface_type))) @interface.def
(import_spec path: (interpreted_string_literal) @import.path)
(call_expression function: (selector_expression field: (field_identifier) @call.name)) @call.expr
(call_expression function: (identifier) @call.name) @call.expr
`,
			".rs": `
(function_item name: (identifier) @func.name) @func.def
(struct_item name: (type_identifier) @struct.name) @struct.def
(impl_item type: (type_identifier) @impl.name) @impl.def
(use_declaration argument: (_) @import.path)
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (field_expression field: (field_identifier) @call.name)) @call.expr
`,
			".py": `
(function_definition name: (identifier) @func.name) @func.def
(class_definition name: (identifier) @class.name) @class.def
(import_from_statement module_name: (dotted_name) @import.path)
(import_statement name: (dotted_name) @import.path)
(call function: (identifier) @call.name) @call.expr
(call function: (attribute attribute: (identifier) @call.name)) @call.expr
`,
			".js": `
(function_declaration name: (identifier) @func.name) @func.def
(function_expression name: (identifier) @func.name) @func.def
(arrow_function) @func.def
(method_definition name: (property_identifier) @method.name) @method.def
(class_declaration name: (identifier) @class.name) @class.def
(import_statement source: (string) @import.path)
(export_statement) @export.def
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (member_expression property: (property_identifier) @call.name)) @call.expr
`,
			".mjs": `
(function_declaration name: (identifier) @func.name) @func.def
(function_expression name: (identifier) @func.name) @func.def
(arrow_function) @func.def
(method_definition name: (property_identifier) @method.name) @method.def
(class_declaration name: (identifier) @class.name) @class.def
(import_statement source: (string) @import.path)
(export_statement) @export.def
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (member_expression property: (property_identifier) @call.name)) @call.expr
`,
			".cjs": `
(function_declaration name: (identifier) @func.name) @func.def
(function_expression name: (identifier) @func.name) @func.def
(arrow_function) @func.def
(method_definition name: (property_identifier) @method.name) @method.def
(class_declaration name: (identifier) @class.name) @class.def
(import_statement source: (string) @import.path)
(export_statement) @export.def
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (member_expression property: (property_identifier) @call.name)) @call.expr
`,
			".ts": `
(function_declaration name: (identifier) @func.name) @func.def
(function_expression name: (identifier) @func.name) @func.def
(arrow_function) @func.def
(method_definition name: (property_identifier) @method.name) @method.def
(class_declaration name: (type_identifier) @class.name) @class.def
(interface_declaration name: (type_identifier) @interface.name) @interface.def
(type_alias_declaration name: (type_identifier) @type.name) @type.def
(import_statement source: (string) @import.path)
(export_statement) @export.def
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (member_expression property: (property_identifier) @call.name)) @call.expr
`,
			".tsx": `
(function_declaration name: (identifier) @func.name) @func.def
(function_expression name: (identifier) @func.name) @func.def
(arrow_function) @func.def
(method_definition name: (property_identifier) @method.name) @method.def
(class_declaration name: (type_identifier) @class.name) @class.def
(interface_declaration name: (type_identifier) @interface.name) @interface.def
(type_alias_declaration name: (type_identifier) @type.name) @type.def
(import_statement source: (string) @import.path)
(export_statement) @export.def
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (member_expression property: (property_identifier) @call.name)) @call.expr
(jsx_element) @jsx.elem
(jsx_self_closing_element) @jsx.elem
`,
			".mts": `
(function_declaration name: (identifier) @func.name) @func.def
(function_expression name: (identifier) @func.name) @func.def
(arrow_function) @func.def
(method_definition name: (property_identifier) @method.name) @method.def
(class_declaration name: (type_identifier) @class.name) @class.def
(interface_declaration name: (type_identifier) @interface.name) @interface.def
(type_alias_declaration name: (type_identifier) @type.name) @type.def
(import_statement source: (string) @import.path)
(export_statement) @export.def
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (member_expression property: (property_identifier) @call.name)) @call.expr
`,
			".cts": `
(function_declaration name: (identifier) @func.name) @func.def
(function_expression name: (identifier) @func.name) @func.def
(arrow_function) @func.def
(method_definition name: (property_identifier) @method.name) @method.def
(class_declaration name: (type_identifier) @class.name) @class.def
(interface_declaration name: (type_identifier) @interface.name) @interface.def
(type_alias_declaration name: (type_identifier) @type.name) @type.def
(import_statement source: (string) @import.path)
(export_statement) @export.def
(call_expression function: (identifier) @call.name) @call.expr
(call_expression function: (member_expression property: (property_identifier) @call.name)) @call.expr
`,
		},
	}
}

func (p *TSParser) Supports(extension string) bool {
	_, ok := p.languages[extension]
	return ok
}

func (p *TSParser) SupportedExtensions() []string {
	extensions := make([]string, 0, len(p.languages))
	for ext := range p.languages {
		extensions = append(extensions, ext)
	}
	return extensions
}

func (p *TSParser) ParseFile(ctx context.Context, path string, content []byte) ([]domain.CodeNode, []domain.CodeRelation, error) {
	result, err := p.ParseFileWithErrors(ctx, path, content)
	if err != nil {
		return nil, nil, err
	}
	return result.Nodes, result.Relations, nil
}

func (p *TSParser) ParseFileWithErrors(ctx context.Context, path string, content []byte) (*ParseResult, error) {
	ext := filepath.Ext(path)
	lang, ok := p.languages[ext]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", ext)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	root := tree.RootNode()

	queryStr, ok := p.queries[ext]
	if !ok {
		return nil, fmt.Errorf("no query defined for: %s", ext)
	}

	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return nil, err
	}

	cursor := sitter.NewQueryCursor()
	cursor.Exec(q, root)

	result := &ParseResult{
		Nodes:     make([]domain.CodeNode, 0),
		Relations: make([]domain.CodeRelation, 0),
		Errors:    make([]ParseError, 0),
	}

	hasErrors := root.HasError()
	if hasErrors {
		result.Errors = p.collectErrors(root, content)
	}

	fileNodeID := path
	fileNode := domain.CodeNode{
		ID:   fileNodeID,
		Name: filepath.Base(path),
		Kind: domain.KindFile,
		Path: path,
	}

	type defInfo struct {
		id    string
		start uint32
		end   uint32
	}
	var defs []defInfo

	allSignatures := ""
	fileHasher := sha256.New()
	fileHasher.Write(content)
	contentHash := hex.EncodeToString(fileHasher.Sum(nil))

	cursor.Exec(q, root)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			captureName := q.CaptureNameForId(capture.Index)
			if !strings.HasSuffix(captureName, ".def") {
				continue
			}

			kind := domain.KindFunction
			if strings.Contains(captureName, "struct") || strings.Contains(captureName, "class") {
				kind = domain.KindStruct
			} else if strings.Contains(captureName, "method") {
				kind = domain.KindMethod
			} else if strings.Contains(captureName, "interface") {
				kind = domain.KindInterface
			}

			name := ""
			for _, c := range match.Captures {
				cn := q.CaptureNameForId(c.Index)
				if strings.HasSuffix(cn, ".name") && strings.HasPrefix(cn, strings.Split(captureName, ".")[0]) {
					name = string(content[c.Node.StartByte():c.Node.EndByte()])
					break
				}
			}

			nodeID := fmt.Sprintf("%s:%s", path, name)
			if name == "" {
				nodeID = fmt.Sprintf("%s:anonymous_%d", path, capture.Node.StartByte())
			}

			signature := ""
			sigNode := capture.Node
			for i := 0; i < int(sigNode.ChildCount()); i++ {
				child := sigNode.Child(i)
				if child.Type() == "block" || child.Type() == "compound_statement" || child.Type() == "statement_block" {
					break
				}
				signature += string(content[child.StartByte():child.EndByte()])
			}
			if signature == "" {
				signature = string(content[capture.Node.StartByte():capture.Node.EndByte()])
			}

			trimmedSignature := strings.TrimSpace(signature)
			allSignatures += trimmedSignature

			nodeHasher := sha256.New()
			nodeHasher.Write([]byte(trimmedSignature))
			sigHash := hex.EncodeToString(nodeHasher.Sum(nil))

			nodeContent := string(content[capture.Node.StartByte():capture.Node.EndByte()])
			nodeContentHasher := sha256.New()
			nodeContentHasher.Write([]byte(nodeContent))
			nodeContentHash := hex.EncodeToString(nodeContentHasher.Sum(nil))

			startLine := int(capture.Node.StartPoint().Row) + 1
			endLine := int(capture.Node.EndPoint().Row) + 1
			startColumn := int(capture.Node.StartPoint().Column) + 1
			endColumn := int(capture.Node.EndPoint().Column) + 1

			result.Nodes = append(result.Nodes, domain.CodeNode{
				ID:            nodeID,
				Name:          name,
				Kind:          kind,
				Path:          path,
				StartLine:     startLine,
				EndLine:       endLine,
				StartColumn:   startColumn,
				EndColumn:     endColumn,
				Signature:     trimmedSignature,
				SignatureHash: sigHash,
				Content:       nodeContent,
				ContentHash:   nodeContentHash,
			})

			defs = append(defs, defInfo{
				id:    nodeID,
				start: capture.Node.StartByte(),
				end:   capture.Node.EndByte(),
			})

			result.Relations = append(result.Relations, domain.CodeRelation{
				From: fileNodeID,
				To:   nodeID,
				Type: domain.RelContains,
			})
		}
	}

	aggHasher := sha256.New()
	aggHasher.Write([]byte(allSignatures))
	fileNode.SignatureHash = hex.EncodeToString(aggHasher.Sum(nil))
	fileNode.ContentHash = contentHash
	result.Nodes = append([]domain.CodeNode{fileNode}, result.Nodes...)

	cursor.Exec(q, root)
	for {
		match, ok := cursor.NextMatch()
		if !ok {
			break
		}
		for _, capture := range match.Captures {
			captureName := q.CaptureNameForId(capture.Index)
			node := capture.Node

			switch captureName {
			case "import.path":
				impPath := string(content[node.StartByte():node.EndByte()])
				impPath = strings.Trim(impPath, "\"`'")
				result.Relations = append(result.Relations, domain.CodeRelation{
					From: fileNodeID,
					To:   impPath,
					Type: domain.RelImports,
				})

			case "call.expr":
				callName := ""
				for _, c := range match.Captures {
					cn := q.CaptureNameForId(c.Index)
					if cn == "call.name" {
						callName = string(content[c.Node.StartByte():c.Node.EndByte()])
						break
					}
				}

				if callName != "" {
					enclosingID := fileNodeID
					for _, d := range defs {
						if node.StartByte() >= d.start && node.EndByte() <= d.end {
							enclosingID = d.id
							break
						}
					}

					result.Relations = append(result.Relations, domain.CodeRelation{
						From: enclosingID,
						To:   callName,
						Type: domain.RelCalls,
					})
				}
			}
		}
	}

	return result, nil
}

func (p *TSParser) collectErrors(root *sitter.Node, content []byte) []ParseError {
	var errors []ParseError

	iterateErrors(root, func(n *sitter.Node) {
		errors = append(errors, ParseError{
			Message: "syntax error",
			Line:    int(n.StartPoint().Row) + 1,
			Column:  int(n.StartPoint().Column) + 1,
		})
	})

	return errors
}

func iterateErrors(node *sitter.Node, callback func(*sitter.Node)) {
	if node == nil {
		return
	}

	if node.IsError() || node.IsMissing() {
		callback(node)
	}

	for i := 0; i < int(node.ChildCount()); i++ {
		iterateErrors(node.Child(i), callback)
	}
}
