package domain

type NodeKind string

const (
	KindFunction  NodeKind = "FUNCTION"
	KindStruct    NodeKind = "STRUCT"
	KindInterface NodeKind = "INTERFACE"
	KindMethod    NodeKind = "METHOD"
	KindVariable  NodeKind = "VARIABLE"
	KindPackage   NodeKind = "PACKAGE"
	KindFile      NodeKind = "FILE"
)

type RelationType string

const (
	RelCalls      RelationType = "CALLS"
	RelImplements RelationType = "IMPLEMENTS"
	RelUses       RelationType = "USES"
	RelDependsOn  RelationType = "DEPENDS_ON"
	RelImports    RelationType = "IMPORTS"
	RelContains   RelationType = "CONTAINS"
)

// CodeNode represents an entity in the codebase (func, struct, etc.)
type CodeNode struct {
	ID            string    `json:"id"`
	ProjectID     string    `json:"project_id"`
	Name          string    `json:"name"`
	Kind          NodeKind  `json:"kind"`
	Path          string    `json:"path"`
	StartLine     int       `json:"start_line"`
	EndLine       int       `json:"end_line"`
	StartColumn   int       `json:"start_column"`
	EndColumn     int       `json:"end_column"`
	Signature     string    `json:"signature"`
	SignatureHash string    `json:"signature_hash,omitempty"`
	Content       string    `json:"content"`
	ContentHash   string    `json:"content_hash,omitempty"`
	Embedding     []float32 `json:"embedding,omitempty"`
}

// CodeRelation connects two CodeNodes
type CodeRelation struct {
	From      string                 `json:"from_id"`
	To        string                 `json:"to_id"`
	ProjectID string                 `json:"project_id"`
	Type      RelationType           `json:"type"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// Snippet represents a piece of code with metadata
type Snippet struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
}

// ImpactReport details the downstream effects of a change
type ImpactReport struct {
	TargetNode       CodeNode       `json:"target_node"`
	AffectedNodes    []AffectedNode `json:"affected_nodes"`
	BlastRadiusScore int            `json:"blast_radius_score"`
}

type AffectedNode struct {
	Node     CodeNode     `json:"node"`
	Relation RelationType `json:"relation"`
	Depth    int          `json:"depth"`
}

// ProjectMap represents the project structure
type ProjectMap struct {
	RootPath string   `json:"root_path"`
	Folders  []string `json:"folders"`
	Files    []string `json:"files"`
}

// DependencyLink tracks third-party dependencies
type DependencyLink struct {
	Package string `json:"package"`
	Version string `json:"version"`
	Type    string `json:"type"` // e.g., "direct", "indirect"
}

// GraphStore defines the interface for RAG operations (to be moved to rag/store later, but here for reference or if domain needs it)
// Actually, interfaces should be in their respective packages as per hexagonal architecture.
// But domain might need to know about some of these for passing data.
