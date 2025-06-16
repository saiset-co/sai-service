package types

type LifecycleManager interface {
	Start() error
	Stop() error
	IsRunning() bool
}
