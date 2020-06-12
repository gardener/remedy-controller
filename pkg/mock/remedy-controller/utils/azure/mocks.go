// Code generated by MockGen. DO NOT EDIT.
// Source: github.wdf.sap.corp/kubernetes/remedy-controller/pkg/utils/azure (interfaces: PublicIPAddressUtils)

// Package azure is a generated GoMock package.
package azure

import (
	context "context"
	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockPublicIPAddressUtils is a mock of PublicIPAddressUtils interface
type MockPublicIPAddressUtils struct {
	ctrl     *gomock.Controller
	recorder *MockPublicIPAddressUtilsMockRecorder
}

// MockPublicIPAddressUtilsMockRecorder is the mock recorder for MockPublicIPAddressUtils
type MockPublicIPAddressUtilsMockRecorder struct {
	mock *MockPublicIPAddressUtils
}

// NewMockPublicIPAddressUtils creates a new mock instance
func NewMockPublicIPAddressUtils(ctrl *gomock.Controller) *MockPublicIPAddressUtils {
	mock := &MockPublicIPAddressUtils{ctrl: ctrl}
	mock.recorder = &MockPublicIPAddressUtilsMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPublicIPAddressUtils) EXPECT() *MockPublicIPAddressUtilsMockRecorder {
	return m.recorder
}

// Delete mocks base method
func (m *MockPublicIPAddressUtils) Delete(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockPublicIPAddressUtilsMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockPublicIPAddressUtils)(nil).Delete), arg0, arg1)
}

// GetAll mocks base method
func (m *MockPublicIPAddressUtils) GetAll(arg0 context.Context) ([]network.PublicIPAddress, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAll", arg0)
	ret0, _ := ret[0].([]network.PublicIPAddress)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAll indicates an expected call of GetAll
func (mr *MockPublicIPAddressUtilsMockRecorder) GetAll(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAll", reflect.TypeOf((*MockPublicIPAddressUtils)(nil).GetAll), arg0)
}

// GetByIP mocks base method
func (m *MockPublicIPAddressUtils) GetByIP(arg0 context.Context, arg1 string) (*network.PublicIPAddress, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByIP", arg0, arg1)
	ret0, _ := ret[0].(*network.PublicIPAddress)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByIP indicates an expected call of GetByIP
func (mr *MockPublicIPAddressUtilsMockRecorder) GetByIP(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByIP", reflect.TypeOf((*MockPublicIPAddressUtils)(nil).GetByIP), arg0, arg1)
}

// GetByName mocks base method
func (m *MockPublicIPAddressUtils) GetByName(arg0 context.Context, arg1 string) (*network.PublicIPAddress, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByName", arg0, arg1)
	ret0, _ := ret[0].(*network.PublicIPAddress)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByName indicates an expected call of GetByName
func (mr *MockPublicIPAddressUtilsMockRecorder) GetByName(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByName", reflect.TypeOf((*MockPublicIPAddressUtils)(nil).GetByName), arg0, arg1)
}

// RemoveFromLoadBalancer mocks base method
func (m *MockPublicIPAddressUtils) RemoveFromLoadBalancer(arg0 context.Context, arg1 []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveFromLoadBalancer", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveFromLoadBalancer indicates an expected call of RemoveFromLoadBalancer
func (mr *MockPublicIPAddressUtilsMockRecorder) RemoveFromLoadBalancer(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveFromLoadBalancer", reflect.TypeOf((*MockPublicIPAddressUtils)(nil).RemoveFromLoadBalancer), arg0, arg1)
}