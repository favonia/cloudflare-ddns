// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/favonia/cloudflare-ddns/internal/monitor (interfaces: Monitor)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	pp "github.com/favonia/cloudflare-ddns/internal/pp"
	gomock "github.com/golang/mock/gomock"
)

// MockMonitor is a mock of Monitor interface.
type MockMonitor struct {
	ctrl     *gomock.Controller
	recorder *MockMonitorMockRecorder
}

// MockMonitorMockRecorder is the mock recorder for MockMonitor.
type MockMonitorMockRecorder struct {
	mock *MockMonitor
}

// NewMockMonitor creates a new mock instance.
func NewMockMonitor(ctrl *gomock.Controller) *MockMonitor {
	mock := &MockMonitor{ctrl: ctrl}
	mock.recorder = &MockMonitorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMonitor) EXPECT() *MockMonitorMockRecorder {
	return m.recorder
}

// DescribeService mocks base method.
func (m *MockMonitor) DescribeService() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DescribeService")
	ret0, _ := ret[0].(string)
	return ret0
}

// DescribeService indicates an expected call of DescribeService.
func (mr *MockMonitorMockRecorder) DescribeService() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DescribeService", reflect.TypeOf((*MockMonitor)(nil).DescribeService))
}

// ExitStatus mocks base method.
func (m *MockMonitor) ExitStatus(arg0 context.Context, arg1 pp.PP, arg2 int) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ExitStatus", arg0, arg1, arg2)
	ret0, _ := ret[0].(bool)
	return ret0
}

// ExitStatus indicates an expected call of ExitStatus.
func (mr *MockMonitorMockRecorder) ExitStatus(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ExitStatus", reflect.TypeOf((*MockMonitor)(nil).ExitStatus), arg0, arg1, arg2)
}

// Failure mocks base method.
func (m *MockMonitor) Failure(arg0 context.Context, arg1 pp.PP) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Failure", arg0, arg1)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Failure indicates an expected call of Failure.
func (mr *MockMonitorMockRecorder) Failure(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Failure", reflect.TypeOf((*MockMonitor)(nil).Failure), arg0, arg1)
}

// Start mocks base method.
func (m *MockMonitor) Start(arg0 context.Context, arg1 pp.PP) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Start", arg0, arg1)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Start indicates an expected call of Start.
func (mr *MockMonitorMockRecorder) Start(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Start", reflect.TypeOf((*MockMonitor)(nil).Start), arg0, arg1)
}

// Success mocks base method.
func (m *MockMonitor) Success(arg0 context.Context, arg1 pp.PP) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Success", arg0, arg1)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Success indicates an expected call of Success.
func (mr *MockMonitorMockRecorder) Success(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Success", reflect.TypeOf((*MockMonitor)(nil).Success), arg0, arg1)
}
