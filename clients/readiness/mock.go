// Code generated by MockGen. DO NOT EDIT.
// Source: client.go

// Package readiness is a generated GoMock package.
package readiness

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockClient is a mock of Client interface
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// WaitForReadiness mocks base method
func (m *MockClient) WaitForReadiness(protocol, host string, port int, path, hostname string, timeoutSeconds int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WaitForReadiness", protocol, host, port, path, hostname, timeoutSeconds)
	ret0, _ := ret[0].(error)
	return ret0
}

// WaitForReadiness indicates an expected call of WaitForReadiness
func (mr *MockClientMockRecorder) WaitForReadiness(protocol, host, port, path, hostname, timeoutSeconds interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForReadiness", reflect.TypeOf((*MockClient)(nil).WaitForReadiness), protocol, host, port, path, hostname, timeoutSeconds)
}
