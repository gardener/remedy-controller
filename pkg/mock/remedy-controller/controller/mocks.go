// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gardener/remedy-controller/pkg/controller (interfaces: Actuator,Mapper)
//
// Generated by this command:
//
//	mockgen -package controller -destination=mocks.go github.com/gardener/remedy-controller/pkg/controller Actuator,Mapper
//

// Package controller is a generated GoMock package.
package controller

import (
	context "context"
	reflect "reflect"
	time "time"

	gomock "go.uber.org/mock/gomock"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// MockActuator is a mock of Actuator interface.
type MockActuator struct {
	ctrl     *gomock.Controller
	recorder *MockActuatorMockRecorder
	isgomock struct{}
}

// MockActuatorMockRecorder is the mock recorder for MockActuator.
type MockActuatorMockRecorder struct {
	mock *MockActuator
}

// NewMockActuator creates a new mock instance.
func NewMockActuator(ctrl *gomock.Controller) *MockActuator {
	mock := &MockActuator{ctrl: ctrl}
	mock.recorder = &MockActuatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockActuator) EXPECT() *MockActuatorMockRecorder {
	return m.recorder
}

// CreateOrUpdate mocks base method.
func (m *MockActuator) CreateOrUpdate(arg0 context.Context, arg1 client.Object) (time.Duration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdate", arg0, arg1)
	ret0, _ := ret[0].(time.Duration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateOrUpdate indicates an expected call of CreateOrUpdate.
func (mr *MockActuatorMockRecorder) CreateOrUpdate(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdate", reflect.TypeOf((*MockActuator)(nil).CreateOrUpdate), arg0, arg1)
}

// Delete mocks base method.
func (m *MockActuator) Delete(arg0 context.Context, arg1 client.Object) (time.Duration, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(time.Duration)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Delete indicates an expected call of Delete.
func (mr *MockActuatorMockRecorder) Delete(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockActuator)(nil).Delete), arg0, arg1)
}

// ShouldFinalize mocks base method.
func (m *MockActuator) ShouldFinalize(arg0 context.Context, arg1 client.Object) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ShouldFinalize", arg0, arg1)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ShouldFinalize indicates an expected call of ShouldFinalize.
func (mr *MockActuatorMockRecorder) ShouldFinalize(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ShouldFinalize", reflect.TypeOf((*MockActuator)(nil).ShouldFinalize), arg0, arg1)
}

// MockMapper is a mock of Mapper interface.
type MockMapper struct {
	ctrl     *gomock.Controller
	recorder *MockMapperMockRecorder
	isgomock struct{}
}

// MockMapperMockRecorder is the mock recorder for MockMapper.
type MockMapperMockRecorder struct {
	mock *MockMapper
}

// NewMockMapper creates a new mock instance.
func NewMockMapper(ctrl *gomock.Controller) *MockMapper {
	mock := &MockMapper{ctrl: ctrl}
	mock.recorder = &MockMapperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMapper) EXPECT() *MockMapperMockRecorder {
	return m.recorder
}

// Map mocks base method.
func (m *MockMapper) Map(obj client.Object) client.ObjectKey {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Map", obj)
	ret0, _ := ret[0].(client.ObjectKey)
	return ret0
}

// Map indicates an expected call of Map.
func (mr *MockMapperMockRecorder) Map(obj any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Map", reflect.TypeOf((*MockMapper)(nil).Map), obj)
}
