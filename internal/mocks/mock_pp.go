// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/favonia/cloudflare-ddns/internal/pp (interfaces: PP)
//
// Generated by this command:
//
//	mockgen -typed -destination=../mocks/mock_pp.go -package=mocks . PP
//
// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	pp "github.com/favonia/cloudflare-ddns/internal/pp"
	gomock "go.uber.org/mock/gomock"
)

// MockPP is a mock of PP interface.
type MockPP struct {
	ctrl     *gomock.Controller
	recorder *MockPPMockRecorder
}

// MockPPMockRecorder is the mock recorder for MockPP.
type MockPPMockRecorder struct {
	mock *MockPP
}

// NewMockPP creates a new mock instance.
func NewMockPP(ctrl *gomock.Controller) *MockPP {
	mock := &MockPP{ctrl: ctrl}
	mock.recorder = &MockPPMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPP) EXPECT() *MockPPMockRecorder {
	return m.recorder
}

// Errorf mocks base method.
func (m *MockPP) Errorf(arg0 pp.Emoji, arg1 string, arg2 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Errorf", varargs...)
}

// Errorf indicates an expected call of Errorf.
func (mr *MockPPMockRecorder) Errorf(arg0, arg1 any, arg2 ...any) *PPErrorfCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Errorf", reflect.TypeOf((*MockPP)(nil).Errorf), varargs...)
	return &PPErrorfCall{Call: call}
}

// PPErrorfCall wrap *gomock.Call
type PPErrorfCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPErrorfCall) Return() *PPErrorfCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPErrorfCall) Do(f func(pp.Emoji, string, ...any)) *PPErrorfCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPErrorfCall) DoAndReturn(f func(pp.Emoji, string, ...any)) *PPErrorfCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// IncIndent mocks base method.
func (m *MockPP) IncIndent() pp.PP {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IncIndent")
	ret0, _ := ret[0].(pp.PP)
	return ret0
}

// IncIndent indicates an expected call of IncIndent.
func (mr *MockPPMockRecorder) IncIndent() *PPIncIndentCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IncIndent", reflect.TypeOf((*MockPP)(nil).IncIndent))
	return &PPIncIndentCall{Call: call}
}

// PPIncIndentCall wrap *gomock.Call
type PPIncIndentCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPIncIndentCall) Return(arg0 pp.PP) *PPIncIndentCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPIncIndentCall) Do(f func() pp.PP) *PPIncIndentCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPIncIndentCall) DoAndReturn(f func() pp.PP) *PPIncIndentCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Infof mocks base method.
func (m *MockPP) Infof(arg0 pp.Emoji, arg1 string, arg2 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Infof", varargs...)
}

// Infof indicates an expected call of Infof.
func (mr *MockPPMockRecorder) Infof(arg0, arg1 any, arg2 ...any) *PPInfofCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Infof", reflect.TypeOf((*MockPP)(nil).Infof), varargs...)
	return &PPInfofCall{Call: call}
}

// PPInfofCall wrap *gomock.Call
type PPInfofCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPInfofCall) Return() *PPInfofCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPInfofCall) Do(f func(pp.Emoji, string, ...any)) *PPInfofCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPInfofCall) DoAndReturn(f func(pp.Emoji, string, ...any)) *PPInfofCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// IsEnabledFor mocks base method.
func (m *MockPP) IsEnabledFor(arg0 pp.Verbosity) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsEnabledFor", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsEnabledFor indicates an expected call of IsEnabledFor.
func (mr *MockPPMockRecorder) IsEnabledFor(arg0 any) *PPIsEnabledForCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsEnabledFor", reflect.TypeOf((*MockPP)(nil).IsEnabledFor), arg0)
	return &PPIsEnabledForCall{Call: call}
}

// PPIsEnabledForCall wrap *gomock.Call
type PPIsEnabledForCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPIsEnabledForCall) Return(arg0 bool) *PPIsEnabledForCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPIsEnabledForCall) Do(f func(pp.Verbosity) bool) *PPIsEnabledForCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPIsEnabledForCall) DoAndReturn(f func(pp.Verbosity) bool) *PPIsEnabledForCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Noticef mocks base method.
func (m *MockPP) Noticef(arg0 pp.Emoji, arg1 string, arg2 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Noticef", varargs...)
}

// Noticef indicates an expected call of Noticef.
func (mr *MockPPMockRecorder) Noticef(arg0, arg1 any, arg2 ...any) *PPNoticefCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Noticef", reflect.TypeOf((*MockPP)(nil).Noticef), varargs...)
	return &PPNoticefCall{Call: call}
}

// PPNoticefCall wrap *gomock.Call
type PPNoticefCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPNoticefCall) Return() *PPNoticefCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPNoticefCall) Do(f func(pp.Emoji, string, ...any)) *PPNoticefCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPNoticefCall) DoAndReturn(f func(pp.Emoji, string, ...any)) *PPNoticefCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetEmoji mocks base method.
func (m *MockPP) SetEmoji(arg0 bool) pp.PP {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetEmoji", arg0)
	ret0, _ := ret[0].(pp.PP)
	return ret0
}

// SetEmoji indicates an expected call of SetEmoji.
func (mr *MockPPMockRecorder) SetEmoji(arg0 any) *PPSetEmojiCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetEmoji", reflect.TypeOf((*MockPP)(nil).SetEmoji), arg0)
	return &PPSetEmojiCall{Call: call}
}

// PPSetEmojiCall wrap *gomock.Call
type PPSetEmojiCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPSetEmojiCall) Return(arg0 pp.PP) *PPSetEmojiCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPSetEmojiCall) Do(f func(bool) pp.PP) *PPSetEmojiCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPSetEmojiCall) DoAndReturn(f func(bool) pp.PP) *PPSetEmojiCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// SetVerbosity mocks base method.
func (m *MockPP) SetVerbosity(arg0 pp.Verbosity) pp.PP {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SetVerbosity", arg0)
	ret0, _ := ret[0].(pp.PP)
	return ret0
}

// SetVerbosity indicates an expected call of SetVerbosity.
func (mr *MockPPMockRecorder) SetVerbosity(arg0 any) *PPSetVerbosityCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetVerbosity", reflect.TypeOf((*MockPP)(nil).SetVerbosity), arg0)
	return &PPSetVerbosityCall{Call: call}
}

// PPSetVerbosityCall wrap *gomock.Call
type PPSetVerbosityCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPSetVerbosityCall) Return(arg0 pp.PP) *PPSetVerbosityCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPSetVerbosityCall) Do(f func(pp.Verbosity) pp.PP) *PPSetVerbosityCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPSetVerbosityCall) DoAndReturn(f func(pp.Verbosity) pp.PP) *PPSetVerbosityCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Warningf mocks base method.
func (m *MockPP) Warningf(arg0 pp.Emoji, arg1 string, arg2 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Warningf", varargs...)
}

// Warningf indicates an expected call of Warningf.
func (mr *MockPPMockRecorder) Warningf(arg0, arg1 any, arg2 ...any) *PPWarningfCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Warningf", reflect.TypeOf((*MockPP)(nil).Warningf), varargs...)
	return &PPWarningfCall{Call: call}
}

// PPWarningfCall wrap *gomock.Call
type PPWarningfCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPWarningfCall) Return() *PPWarningfCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPWarningfCall) Do(f func(pp.Emoji, string, ...any)) *PPWarningfCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPWarningfCall) DoAndReturn(f func(pp.Emoji, string, ...any)) *PPWarningfCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
