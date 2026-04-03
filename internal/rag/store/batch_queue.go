package store

import (
	"context"
	"sync"
	"time"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/domain"
)

type BatchItem struct {
	Nodes       []domain.CodeNode
	Relations   []domain.CodeRelation
	Done        chan error
	SubmittedAt time.Time
}

type BatchQueue struct {
	nodeQueue     chan []domain.CodeNode
	relationQueue chan []domain.CodeRelation
	batchSize     int
	flushInterval time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

func NewBatchQueue(batchSize int) *BatchQueue {
	q := &BatchQueue{
		nodeQueue:     make(chan []domain.CodeNode, batchSize*2),
		relationQueue: make(chan []domain.CodeRelation, batchSize*2),
		batchSize:     batchSize,
		flushInterval: 100 * time.Millisecond,
		stopCh:        make(chan struct{}),
	}
	return q
}

func (q *BatchQueue) Start(ctx context.Context, store *Neo4jStore) {
	q.wg.Add(2)

	go q.processNodeBatch(ctx, store)
	go q.processRelationBatch(ctx, store)
}

func (q *BatchQueue) Stop() {
	close(q.stopCh)
	q.wg.Wait()
}

func (q *BatchQueue) SubmitNodes(ctx context.Context, nodes []domain.CodeNode, store *Neo4jStore) error {
	if len(nodes) == 0 {
		return nil
	}

	if len(nodes) >= q.batchSize {
		return q.executeNodeBatch(ctx, store, nodes)
	}

	select {
	case q.nodeQueue <- nodes:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *BatchQueue) SubmitRelations(ctx context.Context, relations []domain.CodeRelation, store *Neo4jStore) error {
	if len(relations) == 0 {
		return nil
	}

	if len(relations) >= q.batchSize {
		return q.executeRelationBatch(ctx, store, relations)
	}

	select {
	case q.relationQueue <- relations:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (q *BatchQueue) processNodeBatch(ctx context.Context, store *Neo4jStore) {
	defer q.wg.Done()

	buffer := make([]domain.CodeNode, 0, q.batchSize)
	ticker := time.NewTicker(q.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case nodes := <-q.nodeQueue:
			buffer = append(buffer, nodes...)
			if len(buffer) >= q.batchSize {
				if err := q.executeNodeBatch(ctx, store, buffer); err != nil {
					_ = err
				}
				buffer = buffer[:0]
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				if err := q.executeNodeBatch(ctx, store, buffer); err != nil {
					_ = err
				}
				buffer = buffer[:0]
			}
		case <-q.stopCh:
			if len(buffer) > 0 {
				_ = q.executeNodeBatch(ctx, store, buffer)
			}
			return
		case <-ctx.Done():
			if len(buffer) > 0 {
				_ = q.executeNodeBatch(ctx, store, buffer)
			}
			return
		}
	}
}

func (q *BatchQueue) processRelationBatch(ctx context.Context, store *Neo4jStore) {
	defer q.wg.Done()

	buffer := make([]domain.CodeRelation, 0, q.batchSize)
	ticker := time.NewTicker(q.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case relations := <-q.relationQueue:
			buffer = append(buffer, relations...)
			if len(buffer) >= q.batchSize {
				if err := q.executeRelationBatch(ctx, store, buffer); err != nil {
					_ = err
				}
				buffer = buffer[:0]
			}
		case <-ticker.C:
			if len(buffer) > 0 {
				if err := q.executeRelationBatch(ctx, store, buffer); err != nil {
					_ = err
				}
				buffer = buffer[:0]
			}
		case <-q.stopCh:
			if len(buffer) > 0 {
				_ = q.executeRelationBatch(ctx, store, buffer)
			}
			return
		case <-ctx.Done():
			if len(buffer) > 0 {
				_ = q.executeRelationBatch(ctx, store, buffer)
			}
			return
		}
	}
}

func (q *BatchQueue) executeNodeBatch(ctx context.Context, store *Neo4jStore, nodes []domain.CodeNode) error {
	for _, node := range nodes {
		if err := store.UpsertNode(ctx, node); err != nil {
			return err
		}
	}
	return nil
}

func (q *BatchQueue) executeRelationBatch(ctx context.Context, store *Neo4jStore, relations []domain.CodeRelation) error {
	for _, rel := range relations {
		if err := store.UpsertRelation(ctx, rel); err != nil {
			return err
		}
	}
	return nil
}
