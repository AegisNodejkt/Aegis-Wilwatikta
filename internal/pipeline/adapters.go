package pipeline

import (
	"context"

	"github.com/aegis-wilwatikta/ai-reviewer/internal/agents"
)

type ScoutAdapter struct {
	scout *agents.Scout
	owner string
	repo  string
}

func NewScoutAdapter(scout *agents.Scout, owner, repo string) *ScoutAdapter {
	return &ScoutAdapter{
		scout: scout,
		owner: owner,
		repo:  repo}
}

func (a *ScoutAdapter) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	pipelineInput, ok := input.(*PipelineInput)
	if !ok {
		return nil, ErrInvalidInput
	}

	additionalContext, reports, err := a.scout.GatherContext(ctx, a.owner, a.repo, pipelineInput.PR)
	if err != nil {
		return nil, err
	}

	return &ScoutOutput{
		AdditionalContext: additionalContext,
		ImpactReports:     reports,
	}, nil
}

func (a *ScoutAdapter) Name() AgentName {
	return AgentScout
}

type ArchitectAdapter struct {
	architect *agents.Architect
}

func NewArchitectAdapter(architect *agents.Architect) *ArchitectAdapter {
	return &ArchitectAdapter{
		architect: architect}
}

func (a *ArchitectAdapter) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	architectInput, ok := input.(*ArchitectInput)
	if !ok {
		return nil, ErrInvalidInput
	}

	rawReview, err := a.architect.Review(ctx, architectInput.PR, architectInput.AdditionalContext)
	if err != nil {
		return nil, err
	}

	return &ArchitectOutput{
		RawReview: rawReview,
	}, nil
}

func (a *ArchitectAdapter) Name() AgentName {
	return AgentArchitect
}

type DiplomatAdapter struct {
	diplomat *agents.Diplomat
}

func NewDiplomatAdapter(diplomat *agents.Diplomat) *DiplomatAdapter {
	return &DiplomatAdapter{
		diplomat: diplomat}
}

func (a *DiplomatAdapter) Execute(ctx context.Context, input interface{}) (interface{}, error) {
	diplomatInput, ok := input.(*DiplomatInput)
	if !ok {
		return nil, ErrInvalidInput
	}
	aggregated := agents.AggregateImpacts(diplomatInput.ImpactReports)
	healthScore := agents.CalculateHealthScore(aggregated)

	result, err := a.diplomat.FormatReview(ctx, diplomatInput.RawReview, aggregated, healthScore)
	if err != nil {
		return nil, err
	}

	return &DiplomatOutput{
		Result: result,
	}, nil
}

func (a *DiplomatAdapter) Name() AgentName {
	return AgentDiplomat
}
