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
	SetConfig(state action.ListStates, allNameSpaces bool)
}

func NewList(action *action.List) *Lister {
	return &Lister{action}
}

func (l *Lister) SetConfig(state action.ListStates, allNameSpaces bool) {
	l.StateMask = state
	l.AllNamespaces = allNameSpaces
}
