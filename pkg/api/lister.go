package api

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

type list struct {
	*action.List
}

type lister interface {
	Run() ([]*release.Release, error)
	SetStateMask()
	SetState(state action.ListStates)
}

func NewList(action *action.List) *list {
	return &list{action}
}

func (l *list) SetState(state action.ListStates) {
	l.StateMask = state
}
