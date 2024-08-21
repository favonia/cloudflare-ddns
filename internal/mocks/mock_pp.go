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

// Hintf mocks base method.
func (m *MockPP) Hintf(arg0 pp.Hint, arg1 string, arg2 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Hintf", varargs...)
}

// Hintf indicates an expected call of Hintf.
func (mr *MockPPMockRecorder) Hintf(arg0, arg1 any, arg2 ...any) *PPHintfCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1}, arg2...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Hintf", reflect.TypeOf((*MockPP)(nil).Hintf), varargs...)
	return &PPHintfCall{Call: call}
}

// PPHintfCall wrap *gomock.Call
type PPHintfCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPHintfCall) Return() *PPHintfCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPHintfCall) Do(f func(pp.Hint, string, ...any)) *PPHintfCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPHintfCall) DoAndReturn(f func(pp.Hint, string, ...any)) *PPHintfCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}

// Indent mocks base method.
func (m *MockPP) Indent() pp.PP {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Indent")
	ret0, _ := ret[0].(pp.PP)
	return ret0
}

// Indent indicates an expected call of Indent.
func (mr *MockPPMockRecorder) Indent() *PPIndentCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Indent", reflect.TypeOf((*MockPP)(nil).Indent))
	return &PPIndentCall{Call: call}
}

// PPIndentCall wrap *gomock.Call
type PPIndentCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPIndentCall) Return(arg0 pp.PP) *PPIndentCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPIndentCall) Do(f func() pp.PP) *PPIndentCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPIndentCall) DoAndReturn(f func() pp.PP) *PPIndentCall {
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

// IsShowing mocks base method.
func (m *MockPP) IsShowing(arg0 pp.Verbosity) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsShowing", arg0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsShowing indicates an expected call of IsShowing.
func (mr *MockPPMockRecorder) IsShowing(arg0 any) *PPIsShowingCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsShowing", reflect.TypeOf((*MockPP)(nil).IsShowing), arg0)
	return &PPIsShowingCall{Call: call}
}

// PPIsShowingCall wrap *gomock.Call
type PPIsShowingCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPIsShowingCall) Return(arg0 bool) *PPIsShowingCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPIsShowingCall) Do(f func(pp.Verbosity) bool) *PPIsShowingCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPIsShowingCall) DoAndReturn(f func(pp.Verbosity) bool) *PPIsShowingCall {
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

// SuppressHint mocks base method.
func (m *MockPP) SuppressHint(arg0 pp.Hint) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SuppressHint", arg0)
}

// SuppressHint indicates an expected call of SuppressHint.
func (mr *MockPPMockRecorder) SuppressHint(arg0 any) *PPSuppressHintCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SuppressHint", reflect.TypeOf((*MockPP)(nil).SuppressHint), arg0)
	return &PPSuppressHintCall{Call: call}
}

// PPSuppressHintCall wrap *gomock.Call
type PPSuppressHintCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPSuppressHintCall) Return() *PPSuppressHintCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPSuppressHintCall) Do(f func(pp.Hint)) *PPSuppressHintCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPSuppressHintCall) DoAndReturn(f func(pp.Hint)) *PPSuppressHintCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
