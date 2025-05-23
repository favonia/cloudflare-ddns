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

// BlankLineIfVerbose mocks base method.
func (m *MockPP) BlankLineIfVerbose() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "BlankLineIfVerbose")
}

// BlankLineIfVerbose indicates an expected call of BlankLineIfVerbose.
func (mr *MockPPMockRecorder) BlankLineIfVerbose() *PPBlankLineIfVerboseCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BlankLineIfVerbose", reflect.TypeOf((*MockPP)(nil).BlankLineIfVerbose))
	return &PPBlankLineIfVerboseCall{Call: call}
}

// PPBlankLineIfVerboseCall wrap *gomock.Call
type PPBlankLineIfVerboseCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPBlankLineIfVerboseCall) Return() *PPBlankLineIfVerboseCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPBlankLineIfVerboseCall) Do(f func()) *PPBlankLineIfVerboseCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPBlankLineIfVerboseCall) DoAndReturn(f func()) *PPBlankLineIfVerboseCall {
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

// InfoOncef mocks base method.
func (m *MockPP) InfoOncef(arg0 pp.ID, arg1 pp.Emoji, arg2 string, arg3 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "InfoOncef", varargs...)
}

// InfoOncef indicates an expected call of InfoOncef.
func (mr *MockPPMockRecorder) InfoOncef(arg0, arg1, arg2 any, arg3 ...any) *PPInfoOncefCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InfoOncef", reflect.TypeOf((*MockPP)(nil).InfoOncef), varargs...)
	return &PPInfoOncefCall{Call: call}
}

// PPInfoOncefCall wrap *gomock.Call
type PPInfoOncefCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPInfoOncefCall) Return() *PPInfoOncefCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPInfoOncefCall) Do(f func(pp.ID, pp.Emoji, string, ...any)) *PPInfoOncefCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPInfoOncefCall) DoAndReturn(f func(pp.ID, pp.Emoji, string, ...any)) *PPInfoOncefCall {
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

// NoticeOncef mocks base method.
func (m *MockPP) NoticeOncef(arg0 pp.ID, arg1 pp.Emoji, arg2 string, arg3 ...any) {
	m.ctrl.T.Helper()
	varargs := []any{arg0, arg1, arg2}
	for _, a := range arg3 {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "NoticeOncef", varargs...)
}

// NoticeOncef indicates an expected call of NoticeOncef.
func (mr *MockPPMockRecorder) NoticeOncef(arg0, arg1, arg2 any, arg3 ...any) *PPNoticeOncefCall {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0, arg1, arg2}, arg3...)
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NoticeOncef", reflect.TypeOf((*MockPP)(nil).NoticeOncef), varargs...)
	return &PPNoticeOncefCall{Call: call}
}

// PPNoticeOncefCall wrap *gomock.Call
type PPNoticeOncefCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPNoticeOncefCall) Return() *PPNoticeOncefCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPNoticeOncefCall) Do(f func(pp.ID, pp.Emoji, string, ...any)) *PPNoticeOncefCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPNoticeOncefCall) DoAndReturn(f func(pp.ID, pp.Emoji, string, ...any)) *PPNoticeOncefCall {
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

// Suppress mocks base method.
func (m *MockPP) Suppress(arg0 pp.ID) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Suppress", arg0)
}

// Suppress indicates an expected call of Suppress.
func (mr *MockPPMockRecorder) Suppress(arg0 any) *PPSuppressCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Suppress", reflect.TypeOf((*MockPP)(nil).Suppress), arg0)
	return &PPSuppressCall{Call: call}
}

// PPSuppressCall wrap *gomock.Call
type PPSuppressCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *PPSuppressCall) Return() *PPSuppressCall {
	c.Call = c.Call.Return()
	return c
}

// Do rewrite *gomock.Call.Do
func (c *PPSuppressCall) Do(f func(pp.ID)) *PPSuppressCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *PPSuppressCall) DoAndReturn(f func(pp.ID)) *PPSuppressCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
