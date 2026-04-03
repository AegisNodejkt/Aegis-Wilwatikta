package store

import (
	"context"
	"fmt"
	"log"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

type IndexManager struct {
	store        *Neo4jStore
	databaseName string
	indexes      []IndexDefinition
}

type IndexDefinition struct {
	Name        string
	Label       string
	Property    string
	Description string
}

var RequiredIndexes = []IndexDefinition{
	{Name: "code_node_id_idx", Label: "CodeNode", Property: "id", Description: "Index on node ID for fast lookups"},
	{Name: "code_node_project_id_idx", Label: "CodeNode", Property: "project_id", Description: "Index on project ID for project-scoped queries"},
	{Name: "code_node_path_idx", Label: "CodeNode", Property: "path", Description: "Index on file path for file-based queries"},
	{Name: "code_node_name_idx", Label: "CodeNode", Property: "name", Description: "Index on name for function/class name lookups"},
	{Name: "code_node_kind_idx", Label: "CodeNode", Property: "kind", Description: "Index on kind for filtering by node type"},
	{Name: "code_node_signature_hash_idx", Label: "CodeNode", Property: "signature_hash", Description: "Index on signature hash for change detection"},
	{Name: "code_node_content_hash_idx", Label: "CodeNode", Property: "content_hash", Description: "Index on content hash for deduplication"},
}

var VectorIndexes = []IndexDefinition{
	{Name: "code_node_embedding_idx", Label: "CodeNode", Property: "embedding", Description: "Vector index on embedding for semantic similarity"},
}

func NewIndexManager(store *Neo4jStore, databaseName string) *IndexManager {
	return &IndexManager{
		store:        store,
		databaseName: databaseName,
		indexes:      RequiredIndexes,
	}
}

func (m *IndexManager) CreateIndexes(ctx context.Context) error {
	session := m.store.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: m.databaseName,
	})
	defer session.Close(ctx)

	for _, idx := range m.indexes {
		if err := m.createIndex(ctx, session, idx); err != nil {
			log.Printf("Warning: failed to create index %s: %v", idx.Name, err)
		}
	}

	vectorIdx := VectorIndexes[0]
	if err := m.createVectorIndex(ctx, session, vectorIdx); err != nil {
		log.Printf("Warning: failed to create vector index %s: %v (requires Neo4j 5.x+)", vectorIdx.Name, err)
	}

	return nil
}

func (m *IndexManager) createIndex(ctx context.Context, session neo4j.SessionWithContext, idx IndexDefinition) error {
	query := fmt.Sprintf("CREATE INDEX %s IF NOT EXISTS FOR (n:%s) ON (n.%s)",
		idx.Name, idx.Label, idx.Property)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		return tx.Run(ctx, query, nil)
	})

	if err != nil {
		return fmt.Errorf("failed to create index %s: %w", idx.Name, err)
	}

	log.Printf("Created index: %s on %s.%s", idx.Name, idx.Label, idx.Property)
	return nil
}

func (m *IndexManager) createVectorIndex(ctx context.Context, session neo4j.SessionWithContext, idx IndexDefinition) error {
	query := fmt.Sprintf(`CALL db.index.vector.createNodeIndex IF NOT EXISTS
		'%s', '%s', '%s', 1536, 'cosine'`,
		idx.Name, idx.Label, idx.Property)

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		return tx.Run(ctx, query, nil)
	})

	if err != nil {
		return fmt.Errorf("failed to create vector index %s: %w", idx.Name, err)
	}

	log.Printf("Created vector index: %s on %s.%s", idx.Name, idx.Label, idx.Property)
	return nil
}

func (m *IndexManager) DropIndex(ctx context.Context, name string) error {
	session := m.store.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeWrite,
		DatabaseName: m.databaseName,
	})
	defer session.Close(ctx)

	query := fmt.Sprintf("DROP INDEX %s IF EXISTS", name)
	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		return tx.Run(ctx, query, nil)
	})

	return err
}

func (m *IndexManager) ListIndexes(ctx context.Context) ([]map[string]interface{}, error) {
	session := m.store.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode:   neo4j.AccessModeRead,
		DatabaseName: m.databaseName,
	})
	defer session.Close(ctx)

	query := "SHOW INDEXES YIELD name, labelsOrTypes, properties, type"

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, query, nil)
		if err != nil {
			return nil, err
		}

		var indexes []map[string]interface{}
		for res.Next(ctx) {
			record := res.Record()
			name, _ := record.Get("name")
			labels, _ := record.Get("labelsOrTypes")
			properties, _ := record.Get("properties")
			idxType, _ := record.Get("type")
			indexes = append(indexes, map[string]interface{}{
				"name":       name,
				"labels":     labels,
				"properties": properties,
				"type":       idxType,
			})
		}
		return indexes, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]map[string]interface{}), nil
}

func (m *IndexManager) VerifyIndexes(ctx context.Context) (map[string]bool, error) {
	existing, err := m.ListIndexes(ctx)
	if err != nil {
		return nil, err
	}

	existingNames := make(map[string]bool)
	for _, idx := range existing {
		if name, ok := idx["name"].(string); ok {
			existingNames[name] = true
		}
	}

	status := make(map[string]bool)
	for _, required := range m.indexes {
		status[required.Name] = existingNames[required.Name]
	}
	status[VectorIndexes[0].Name] = existingNames[VectorIndexes[0].Name]

	return status, nil
}
