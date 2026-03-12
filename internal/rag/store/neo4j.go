package store

import (
	"context"
	"fmt"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
)

type Neo4jStore struct {
	driver       neo4j.DriverWithContext
	databaseName string
}

func NewNeo4jStore(uri, username, password, databaseName string) (*Neo4jStore, error) {
	fmt.Printf("uri: %s, username: %s, password: %s, databaseName: %s\n", uri, username, password, databaseName)
	driver, err := neo4j.NewDriver(uri, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, err
	}
	return &Neo4jStore{driver: driver, databaseName: databaseName}, nil
}

func (s *Neo4jStore) UpsertNode(ctx context.Context, node domain.CodeNode) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: s.databaseName})
	defer session.Close(ctx)

	query := `
	MERGE (n:CodeNode {id: $id, project_id: $project_id})
	SET n.name = $name,
	    n.kind = $kind,
	    n.path = $path,
	    n.signature = $signature,
	    n.signature_hash = $signature_hash,
	    n.content = $content,
	    n.content_hash = $content_hash,
	    n.embedding = $embedding
	`
	params := map[string]interface{}{
		"id":             node.ID,
		"project_id":     node.ProjectID,
		"name":           node.Name,
		"kind":           string(node.Kind),
		"path":           node.Path,
		"signature":      node.Signature,
		"signature_hash": node.SignatureHash,
		"content":        node.Content,
		"content_hash":   node.ContentHash,
		"embedding":      node.Embedding,
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		return tx.Run(ctx, query, params)
	})
	fmt.Printf("UpsertNode: %v\n", err)
	return err
}

func (s *Neo4jStore) UpsertRelation(ctx context.Context, rel domain.CodeRelation) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: s.databaseName})
	defer session.Close(ctx)

	// If To is a name (no colon), try to find a node with that name
	query := fmt.Sprintf(`
	MATCH (a:CodeNode {id: $from, project_id: $project_id})
	WITH a
	MATCH (b:CodeNode {project_id: $project_id})
	WHERE b.id = $to OR b.name = $to
	MERGE (a)-[r:%s {project_id: $project_id}]->(b)
	`, rel.Type)

	params := map[string]interface{}{
		"from":       rel.From,
		"to":         rel.To,
		"project_id": rel.ProjectID,
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		return tx.Run(ctx, query, params)
	})
	return err
}

func (s *Neo4jStore) GetImpactContext(ctx context.Context, projectID, filePath string) (*domain.ImpactReport, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead, DatabaseName: s.databaseName})
	defer session.Close(ctx)

	query := `
	MATCH (target:CodeNode {path: $path, project_id: $project_id})
	WHERE target.kind <> 'FILE'
	OPTIONAL MATCH (affected:CodeNode {project_id: $project_id})-[r:CALLS|IMPLEMENTS|USES*1..2]->(target)
	WITH target, affected, r
	OPTIONAL MATCH (all_affected:CodeNode {project_id: $project_id})-[:CALLS|IMPLEMENTS|USES]->(target)
	RETURN
		target,
		collect({
			node: affected,
			relation: type(r[0]),
			depth: length(r)
		}) AS impact_list,
		count(DISTINCT all_affected) AS blast_radius
	`
	params := map[string]interface{}{
		"path":       filePath,
		"project_id": projectID,
	}

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var finalReport *domain.ImpactReport
		for res.Next(ctx) {
			record := res.Record()
			targetMap, _ := record.Get("target")
			impactListRaw, _ := record.Get("impact_list")

			targetNode := s.mapNode(targetMap.(neo4j.Node))
			blastRadius, _ := record.Get("blast_radius")

			if finalReport == nil {
				finalReport = &domain.ImpactReport{
					TargetNode:       targetNode,
					BlastRadiusScore: int(blastRadius.(int64)),
				}
			}

			impactList := impactListRaw.([]interface{})
			for _, item := range impactList {
				m := item.(map[string]interface{})
				affectedNodeRaw := m["node"]
				if affectedNodeRaw == nil {
					continue
				}

				affectedNode := s.mapNode(affectedNodeRaw.(neo4j.Node))
				relation, _ := m["relation"].(string)
				depth, _ := m["depth"].(int64)

				finalReport.AffectedNodes = append(finalReport.AffectedNodes, domain.AffectedNode{
					Node:     affectedNode,
					Relation: domain.RelationType(relation),
					Depth:    int(depth),
				})
			}
		}
		if finalReport == nil {
			return nil, fmt.Errorf("no entities found in file %s", filePath)
		}
		return finalReport, nil
	})

	if err != nil {
		return nil, err
	}
	return result.(*domain.ImpactReport), nil
}

func (s *Neo4jStore) QueryContext(ctx context.Context, projectID, filePath string) ([]domain.CodeNode, error) {
	return nil, nil
}

func (s *Neo4jStore) GetFileHash(ctx context.Context, projectID, path string) (string, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead, DatabaseName: s.databaseName})
	defer session.Close(ctx)

	query := `MATCH (f:CodeNode {id: $id, kind: 'FILE', project_id: $project_id}) RETURN f.signature_hash AS hash`
	params := map[string]interface{}{
		"id":         path,
		"project_id": projectID,
	}

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return "", err
		}
		if res.Next(ctx) {
			val, _ := res.Record().Get("hash")
			if s, ok := val.(string); ok {
				return s, nil
			}
		}
		return "", nil
	})
	if err != nil {
		return "", err
	}
	return result.(string), nil
}

func (s *Neo4jStore) DeleteNodesByFile(ctx context.Context, projectID, path string) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: s.databaseName})
	defer session.Close(ctx)

	query := `
	MATCH (n:CodeNode {path: $path, project_id: $project_id})
	WHERE n.kind <> 'FILE'
	DETACH DELETE n
	`
	params := map[string]interface{}{
		"path":       path,
		"project_id": projectID,
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		return tx.Run(ctx, query, params)
	})
	return err
}

func (s *Neo4jStore) DeleteNodesByProject(ctx context.Context, projectID string) error {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite, DatabaseName: s.databaseName})
	defer session.Close(ctx)

	query := `
	MATCH (n:CodeNode {project_id: $project_id})
	DETACH DELETE n
	`
	params := map[string]interface{}{
		"project_id": projectID,
	}

	_, err := session.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		return tx.Run(ctx, query, params)
	})
	return err
}

func (s *Neo4jStore) FindRelatedByEmbedding(ctx context.Context, projectID string, embedding []float32, limit int) ([]domain.CodeNode, error) {
	session := s.driver.NewSession(ctx, neo4j.SessionConfig{AccessMode: neo4j.AccessModeRead, DatabaseName: s.databaseName})
	defer session.Close(ctx)

	// Note: This requires Neo4j 5.x with Vector index.
	// This is a placeholder for a generic similarity query if index is not present.
	query := `
	MATCH (n:CodeNode {project_id: $project_id})
	WHERE n.embedding IS NOT NULL
	RETURN n, gds.similarity.cosine(n.embedding, $embedding) AS score
	ORDER BY score DESC
	LIMIT $limit
	`
	params := map[string]interface{}{
		"embedding":  embedding,
		"limit":      limit,
		"project_id": projectID,
	}

	result, err := session.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (interface{}, error) {
		res, err := tx.Run(ctx, query, params)
		if err != nil {
			return nil, err
		}

		var nodes []domain.CodeNode
		for res.Next(ctx) {
			record := res.Record()
			n, _ := record.Get("n")
			nodes = append(nodes, s.mapNode(n.(neo4j.Node)))
		}
		return nodes, nil
	})

	if err != nil {
		return nil, err
	}
	return result.([]domain.CodeNode), nil
}

func (s *Neo4jStore) mapNode(n neo4j.Node) domain.CodeNode {
	props := n.GetProperties()
	return domain.CodeNode{
		ID:            s.getString(props, "id"),
		Name:          s.getString(props, "name"),
		Kind:          domain.NodeKind(s.getString(props, "kind")),
		Path:          s.getString(props, "path"),
		Signature:     s.getString(props, "signature"),
		SignatureHash: s.getString(props, "signature_hash"),
		Content:       s.getString(props, "content"),
		ContentHash:   s.getString(props, "content_hash"),
	}
}

func (s *Neo4jStore) getString(props map[string]interface{}, key string) string {
	if val, ok := props[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

func (s *Neo4jStore) Close(ctx context.Context) error {
	return s.driver.Close(ctx)
}
