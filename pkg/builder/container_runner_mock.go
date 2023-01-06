// Code generated by MockGen. DO NOT EDIT.
// Source: container_runner.go

// Package mock_builder is a generated GoMock package.
package builder

import (
	context "context"
	reflect "reflect"

	contracts "github.com/estafette/estafette-ci-contracts"
	manifest "github.com/estafette/estafette-ci-manifest"
	gomock "github.com/golang/mock/gomock"
)

// MockContainerRunner is a mock of ContainerRunner interface.
type MockContainerRunner struct {
	ctrl     *gomock.Controller
	recorder *MockContainerRunnerMockRecorder
}

// MockContainerRunnerMockRecorder is the mock recorder for MockContainerRunner.
type MockContainerRunnerMockRecorder struct {
	mock *MockContainerRunner
}

// NewMockContainerRunner creates a new mock instance.
func NewMockContainerRunner(ctrl *gomock.Controller) *MockContainerRunner {
	mock := &MockContainerRunner{ctrl: ctrl}
	mock.recorder = &MockContainerRunnerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockContainerRunner) EXPECT() *MockContainerRunnerMockRecorder {
	return m.recorder
}

// CreateDockerClient mocks base method.
func (m *MockContainerRunner) CreateDockerClient() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateDockerClient")
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateDockerClient indicates an expected call of CreateDockerClient.
func (mr *MockContainerRunnerMockRecorder) CreateDockerClient() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateDockerClient", reflect.TypeOf((*MockContainerRunner)(nil).CreateDockerClient))
}

// CreateNetworks mocks base method.
func (m *MockContainerRunner) CreateNetworks(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateNetworks", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// CreateNetworks indicates an expected call of CreateNetworks.
func (mr *MockContainerRunnerMockRecorder) CreateNetworks(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateNetworks", reflect.TypeOf((*MockContainerRunner)(nil).CreateNetworks), ctx)
}

// DeleteNetworks mocks base method.
func (m *MockContainerRunner) DeleteNetworks(ctx context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteNetworks", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteNetworks indicates an expected call of DeleteNetworks.
func (mr *MockContainerRunnerMockRecorder) DeleteNetworks(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNetworks", reflect.TypeOf((*MockContainerRunner)(nil).DeleteNetworks), ctx)
}

// GetImageSize mocks base method.
func (m *MockContainerRunner) GetImageSize(ctx context.Context, containerImage string) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetImageSize", ctx, containerImage)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetImageSize indicates an expected call of GetImageSize.
func (mr *MockContainerRunnerMockRecorder) GetImageSize(ctx, containerImage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetImageSize", reflect.TypeOf((*MockContainerRunner)(nil).GetImageSize), ctx, containerImage)
}

// HasInjectedCredentials mocks base method.
func (m *MockContainerRunner) HasInjectedCredentials(stageName, containerImage string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasInjectedCredentials", stageName, containerImage)
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasInjectedCredentials indicates an expected call of HasInjectedCredentials.
func (mr *MockContainerRunnerMockRecorder) HasInjectedCredentials(stageName, containerImage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasInjectedCredentials", reflect.TypeOf((*MockContainerRunner)(nil).HasInjectedCredentials), stageName, containerImage)
}

// Info mocks base method.
func (m *MockContainerRunner) Info(ctx context.Context) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Info", ctx)
	ret0, _ := ret[0].(string)
	return ret0
}

// Info indicates an expected call of Info.
func (mr *MockContainerRunnerMockRecorder) Info(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockContainerRunner)(nil).Info), ctx)
}

// IsImagePulled mocks base method.
func (m *MockContainerRunner) IsImagePulled(ctx context.Context, stageName, containerImage string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsImagePulled", ctx, stageName, containerImage)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsImagePulled indicates an expected call of IsImagePulled.
func (mr *MockContainerRunnerMockRecorder) IsImagePulled(ctx, stageName, containerImage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsImagePulled", reflect.TypeOf((*MockContainerRunner)(nil).IsImagePulled), ctx, stageName, containerImage)
}

// IsTrustedImage mocks base method.
func (m *MockContainerRunner) IsTrustedImage(stageName, containerImage string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsTrustedImage", stageName, containerImage)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsTrustedImage indicates an expected call of IsTrustedImage.
func (mr *MockContainerRunnerMockRecorder) IsTrustedImage(stageName, containerImage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsTrustedImage", reflect.TypeOf((*MockContainerRunner)(nil).IsTrustedImage), stageName, containerImage)
}

// PullImage mocks base method.
func (m *MockContainerRunner) PullImage(ctx context.Context, stageName, parentStageName, containerImage string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PullImage", ctx, stageName, parentStageName, containerImage)
	ret0, _ := ret[0].(error)
	return ret0
}

// PullImage indicates an expected call of PullImage.
func (mr *MockContainerRunnerMockRecorder) PullImage(ctx, stageName, parentStageName, containerImage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PullImage", reflect.TypeOf((*MockContainerRunner)(nil).PullImage), ctx, stageName, parentStageName, containerImage)
}

// RunReadinessProbeContainer mocks base method.
func (m *MockContainerRunner) RunReadinessProbeContainer(ctx context.Context, parentStage manifest.EstafetteStage, service manifest.EstafetteService, readiness manifest.ReadinessProbe) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RunReadinessProbeContainer", ctx, parentStage, service, readiness)
	ret0, _ := ret[0].(error)
	return ret0
}

// RunReadinessProbeContainer indicates an expected call of RunReadinessProbeContainer.
func (mr *MockContainerRunnerMockRecorder) RunReadinessProbeContainer(ctx, parentStage, service, readiness interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RunReadinessProbeContainer", reflect.TypeOf((*MockContainerRunner)(nil).RunReadinessProbeContainer), ctx, parentStage, service, readiness)
}

// StartDockerDaemon mocks base method.
func (m *MockContainerRunner) StartDockerDaemon() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartDockerDaemon")
	ret0, _ := ret[0].(error)
	return ret0
}

// StartDockerDaemon indicates an expected call of StartDockerDaemon.
func (mr *MockContainerRunnerMockRecorder) StartDockerDaemon() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartDockerDaemon", reflect.TypeOf((*MockContainerRunner)(nil).StartDockerDaemon))
}

// StartServiceContainer mocks base method.
func (m *MockContainerRunner) StartServiceContainer(ctx context.Context, envvars map[string]string, service manifest.EstafetteService) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartServiceContainer", ctx, envvars, service)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartServiceContainer indicates an expected call of StartServiceContainer.
func (mr *MockContainerRunnerMockRecorder) StartServiceContainer(ctx, envvars, service interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartServiceContainer", reflect.TypeOf((*MockContainerRunner)(nil).StartServiceContainer), ctx, envvars, service)
}

// StartStageContainer mocks base method.
func (m *MockContainerRunner) StartStageContainer(ctx context.Context, depth int, dir string, envvars map[string]string, stage manifest.EstafetteStage, stageIndex int) (string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartStageContainer", ctx, depth, dir, envvars, stage, stageIndex)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartStageContainer indicates an expected call of StartStageContainer.
func (mr *MockContainerRunnerMockRecorder) StartStageContainer(ctx, depth, dir, envvars, stage, stageIndex interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartStageContainer", reflect.TypeOf((*MockContainerRunner)(nil).StartStageContainer), ctx, depth, dir, envvars, stage, stageIndex)
}

// StopAllContainers mocks base method.
func (m *MockContainerRunner) StopAllContainers(ctx context.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StopAllContainers", ctx)
}

// StopAllContainers indicates an expected call of StopAllContainers.
func (mr *MockContainerRunnerMockRecorder) StopAllContainers(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopAllContainers", reflect.TypeOf((*MockContainerRunner)(nil).StopAllContainers), ctx)
}

// StopMultiStageServiceContainers mocks base method.
func (m *MockContainerRunner) StopMultiStageServiceContainers(ctx context.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StopMultiStageServiceContainers", ctx)
}

// StopMultiStageServiceContainers indicates an expected call of StopMultiStageServiceContainers.
func (mr *MockContainerRunnerMockRecorder) StopMultiStageServiceContainers(ctx interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopMultiStageServiceContainers", reflect.TypeOf((*MockContainerRunner)(nil).StopMultiStageServiceContainers), ctx)
}

// StopSingleStageServiceContainers mocks base method.
func (m *MockContainerRunner) StopSingleStageServiceContainers(ctx context.Context, parentStage manifest.EstafetteStage) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "StopSingleStageServiceContainers", ctx, parentStage)
}

// StopSingleStageServiceContainers indicates an expected call of StopSingleStageServiceContainers.
func (mr *MockContainerRunnerMockRecorder) StopSingleStageServiceContainers(ctx, parentStage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StopSingleStageServiceContainers", reflect.TypeOf((*MockContainerRunner)(nil).StopSingleStageServiceContainers), ctx, parentStage)
}

// TailContainerLogs mocks base method.
func (m *MockContainerRunner) TailContainerLogs(ctx context.Context, containerID, parentStageName, stageName string, stageType contracts.LogType, depth int, multiStage *bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TailContainerLogs", ctx, containerID, parentStageName, stageName, stageType, depth, multiStage)
	ret0, _ := ret[0].(error)
	return ret0
}

// TailContainerLogs indicates an expected call of TailContainerLogs.
func (mr *MockContainerRunnerMockRecorder) TailContainerLogs(ctx, containerID, parentStageName, stageName, stageType, depth, multiStage interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TailContainerLogs", reflect.TypeOf((*MockContainerRunner)(nil).TailContainerLogs), ctx, containerID, parentStageName, stageName, stageType, depth, multiStage)
}

// WaitForDockerDaemon mocks base method.
func (m *MockContainerRunner) WaitForDockerDaemon() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "WaitForDockerDaemon")
}

// WaitForDockerDaemon indicates an expected call of WaitForDockerDaemon.
func (mr *MockContainerRunnerMockRecorder) WaitForDockerDaemon() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WaitForDockerDaemon", reflect.TypeOf((*MockContainerRunner)(nil).WaitForDockerDaemon))
}
