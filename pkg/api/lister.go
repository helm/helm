package api

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

type Lister struct {
	*action.List
}

type ListRunner interface {
	Run() ([]*release.Release, error)
	SetStateMask()
	SetState(state action.ListStates)
}

func NewList(action *action.List) *Lister {
	return &Lister{action}
}

func (l *Lister) SetState(state action.ListStates) {
	l.StateMask = state
}
