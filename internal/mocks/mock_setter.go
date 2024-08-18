// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/favonia/cloudflare-ddns/internal/setter (interfaces: Setter)
//
// Generated by this command:
//
//	mockgen -typed -destination=../mocks/mock_setter.go -package=mocks . Setter
//
// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	netip "net/netip"
	reflect "reflect"

	api "github.com/favonia/cloudflare-ddns/internal/api"
	domain "github.com/favonia/cloudflare-ddns/internal/domain"
	ipnet "github.com/favonia/cloudflare-ddns/internal/ipnet"
	pp "github.com/favonia/cloudflare-ddns/internal/pp"
	setter "github.com/favonia/cloudflare-ddns/internal/setter"
	gomock "go.uber.org/mock/gomock"
)

// MockSetter is a mock of Setter interface.
type MockSetter struct {
	ctrl     *gomock.Controller
	recorder *MockSetterMockRecorder
}

// MockSetterMockRecorder is the mock recorder for MockSetter.
type MockSetterMockRecorder struct {
	mock *MockSetter
}

// NewMockSetter creates a new mock instance.
func NewMockSetter(ctrl *gomock.Controller) *MockSetter {
	mock := &MockSetter{ctrl: ctrl}
	mock.recorder = &MockSetterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSetter) EXPECT() *MockSetterMockRecorder {
	return m.recorder
}

// Delete mocks base method.
func (m *MockSetter) Delete(arg0 context.Context, arg1 pp.PP, arg2 ipnet.Type, arg3 domain.Domain) setter.ResponseCode {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(setter.ResponseCode)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockSetterMockRecorder) Delete(arg0, arg1, arg2, arg3 any) *SetterDeleteCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockSetter)(nil).Delete), arg0, arg1, arg2, arg3)
	return &SetterDeleteCall{Call: call}
}

// SetterDeleteCall wrap *gomock.Call
type SetterDeleteCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *SetterDeleteCall) Return(arg0 setter.ResponseCode) *SetterDeleteCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *SetterDeleteCall) Do(f func(context.Context, pp.PP, ipnet.Type, domain.Domain) setter.ResponseCode) *SetterDeleteCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *SetterDeleteCall) DoAndReturn(f func(context.Context, pp.PP, ipnet.Type, domain.Domain) setter.ResponseCode) *SetterDeleteCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// DeleteWAFList mocks base method.
func (m *MockSetter) DeleteWAFList(arg0 context.Context, arg1 pp.PP, arg2 api.WAFList) setter.ResponseCode {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteWAFList", arg0, arg1, arg2)
	ret0, _ := ret[0].(setter.ResponseCode)
	return ret0
}

// DeleteWAFList indicates an expected call of DeleteWAFList.
func (mr *MockSetterMockRecorder) DeleteWAFList(arg0, arg1, arg2 any) *SetterDeleteWAFListCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteWAFList", reflect.TypeOf((*MockSetter)(nil).DeleteWAFList), arg0, arg1, arg2)
	return &SetterDeleteWAFListCall{Call: call}
}

// SetterDeleteWAFListCall wrap *gomock.Call
type SetterDeleteWAFListCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *SetterDeleteWAFListCall) Return(arg0 setter.ResponseCode) *SetterDeleteWAFListCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *SetterDeleteWAFListCall) Do(f func(context.Context, pp.PP, api.WAFList) setter.ResponseCode) *SetterDeleteWAFListCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *SetterDeleteWAFListCall) DoAndReturn(f func(context.Context, pp.PP, api.WAFList) setter.ResponseCode) *SetterDeleteWAFListCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Set mocks base method.
func (m *MockSetter) Set(arg0 context.Context, arg1 pp.PP, arg2 ipnet.Type, arg3 domain.Domain, arg4 netip.Addr, arg5 api.TTL, arg6 bool, arg7 string) setter.ResponseCode {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Set", arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7)
	ret0, _ := ret[0].(setter.ResponseCode)
	return ret0
}

// Set indicates an expected call of Set.
func (mr *MockSetterMockRecorder) Set(arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7 any) *SetterSetCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Set", reflect.TypeOf((*MockSetter)(nil).Set), arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7)
	return &SetterSetCall{Call: call}
}

// SetterSetCall wrap *gomock.Call
type SetterSetCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *SetterSetCall) Return(arg0 setter.ResponseCode) *SetterSetCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *SetterSetCall) Do(f func(context.Context, pp.PP, ipnet.Type, domain.Domain, netip.Addr, api.TTL, bool, string) setter.ResponseCode) *SetterSetCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *SetterSetCall) DoAndReturn(f func(context.Context, pp.PP, ipnet.Type, domain.Domain, netip.Addr, api.TTL, bool, string) setter.ResponseCode) *SetterSetCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetWAFList mocks base method.
func (m *MockSetter) SetWAFList(arg0 context.Context, arg1 pp.PP, arg2 api.WAFList, arg3 string, arg4 map[ipnet.Type]netip.Addr, arg5 string) setter.ResponseCode {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetWAFList", arg0, arg1, arg2, arg3, arg4, arg5)
	ret0, _ := ret[0].(setter.ResponseCode)
	return ret0
}

// SetWAFList indicates an expected call of SetWAFList.
func (mr *MockSetterMockRecorder) SetWAFList(arg0, arg1, arg2, arg3, arg4, arg5 any) *SetterSetWAFListCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetWAFList", reflect.TypeOf((*MockSetter)(nil).SetWAFList), arg0, arg1, arg2, arg3, arg4, arg5)
	return &SetterSetWAFListCall{Call: call}
}

// SetterSetWAFListCall wrap *gomock.Call
type SetterSetWAFListCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *SetterSetWAFListCall) Return(arg0 setter.ResponseCode) *SetterSetWAFListCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *SetterSetWAFListCall) Do(f func(context.Context, pp.PP, api.WAFList, string, map[ipnet.Type]netip.Addr, string) setter.ResponseCode) *SetterSetWAFListCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *SetterSetWAFListCall) DoAndReturn(f func(context.Context, pp.PP, api.WAFList, string, map[ipnet.Type]netip.Addr, string) setter.ResponseCode) *SetterSetWAFListCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
