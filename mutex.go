package readline

import "sync"

type MultiMuTex struct {
	useMutex sync.RWMutex // 主锁

	checkMutex    sync.RWMutex //check锁，判断当前是否上锁了
	useLockStatus bool
}

func (m *MultiMuTex) Lock() {
	m.useMutex.Lock()

	m.checkMutex.Lock()
	defer m.checkMutex.Unlock()
	m.useLockStatus = true
}

func (m *MultiMuTex) Unlock() {
	m.useMutex.Unlock()

	m.checkMutex.Lock()
	defer m.checkMutex.Unlock()
	m.useLockStatus = false
}

func (m *MultiMuTex) CheckLockStatus() bool {
	m.checkMutex.RLock()
	defer m.checkMutex.RUnlock()
	return m.useLockStatus
}

func (m *MultiMuTex) Wait() {
	m.useMutex.Lock()
	m.useMutex.Unlock()
}
