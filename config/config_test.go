package config

import "github.com/stretchr/testify/mock"


type MockConfig struct {
	mock.Mock 
}

func (m *MockConfig) GetNodebyHeight(height uint64) *Node {
	args := m.Called(height) 
	if node, ok := args.Get(0).(*Node); ok {
		return node
	}
	return nil
}


