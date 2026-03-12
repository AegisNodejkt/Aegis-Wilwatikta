package parser

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
	"github.com/smacker/go-tree-sitter/rust"
)

type TSParser struct {
	languages map[string]*sitter.Language
	queries   map[string]string
}

func NewTSParser() *TSParser {
	return &TSParser{
		languages: map[string]*sitter.Language{
			".go": golang.GetLanguage(),
			".rs": rust.GetLanguage(),
			".py": python.GetLanguage(),
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
		},
	}
}

func (p *TSParser) Supports(extension string) bool {
	_, ok := p.languages[extension]
	return ok
}

func (p *TSParser) ParseFile(ctx context.Context, path string, content []byte) ([]domain.CodeNode, []domain.CodeRelation, error) {
	ext := filepath.Ext(path)
	lang, ok := p.languages[ext]
	if !ok {
		return nil, nil, fmt.Errorf("unsupported language: %s", ext)
	}

	parser := sitter.NewParser()
	parser.SetLanguage(lang)
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, nil, err
	}
	defer tree.Close()
	root := tree.RootNode()

	queryStr, ok := p.queries[ext]
	if !ok {
		return nil, nil, fmt.Errorf("no query defined for: %s", ext)
	}

	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return nil, nil, err
	}

	cursor := sitter.NewQueryCursor()
	cursor.Exec(q, root)

	var nodes []domain.CodeNode
	var relations []domain.CodeRelation

	// Add file node
	fileNodeID := path
	nodes = append(nodes, domain.CodeNode{
		ID:   fileNodeID,
		Name: filepath.Base(path),
		Kind: domain.KindFile,
		Path: path,
	})

	type defInfo struct {
		id    string
		start uint32
		end   uint32
	}
	var defs []defInfo

	// First pass: collect all definitions
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

			signature := ""
			// Refine signature extraction (take first line or specific parts)
			sigNode := capture.Node
			for i := 0; i < int(sigNode.ChildCount()); i++ {
				child := sigNode.Child(i)
				if child.Type() == "block" || child.Type() == "compound_statement" {
					break
				}
				signature += string(content[child.StartByte():child.EndByte()])
			}
			if signature == "" {
				signature = string(content[capture.Node.StartByte():capture.Node.EndByte()])
			}

			nodes = append(nodes, domain.CodeNode{
				ID:        nodeID,
				Name:      name,
				Kind:      kind,
				Path:      path,
				Signature: strings.TrimSpace(signature),
				Content:   string(content[capture.Node.StartByte():capture.Node.EndByte()]),
			})

			defs = append(defs, defInfo{
				id:    nodeID,
				start: capture.Node.StartByte(),
				end:   capture.Node.EndByte(),
			})

			// Link file to entity
			relations = append(relations, domain.CodeRelation{
				From: fileNodeID,
				To:   nodeID,
				Type: domain.RelContains,
			})
		}
	}

	// Second pass: collect relations (imports and calls)
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
				relations = append(relations, domain.CodeRelation{
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
					// Find enclosing definition
					enclosingID := fileNodeID
					for _, d := range defs {
						if node.StartByte() >= d.start && node.EndByte() <= d.end {
							enclosingID = d.id
							break
						}
					}

					relations = append(relations, domain.CodeRelation{
						From: enclosingID,
						To:   callName, // Still just a name, will handle in store
						Type: domain.RelCalls,
					})
				}
			}
		}
	}

	return nodes, relations, nil
}
