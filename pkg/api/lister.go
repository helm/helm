package api

import (
	"github.com/stretchr/testify/mock"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

type list struct {
	*action.List
}

type lister interface {
	Run() ([]*release.Release, error)
	SetStateMask()
}

func NewList(action *action.List) *list{
	return &list{action}
}


type MockList struct{ mock.Mock }

func (m *MockList) Run() ([]*release.Release, error) {
	args := m.Called()
	return args.Get(0).([]*release.Release), args.Error(1)
}

func (m *MockList) SetStateMask() {
	m.Called()
}