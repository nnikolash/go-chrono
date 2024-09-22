package chrono

type RWLocker interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

type NoLock struct {
	locked bool
	rlocks int
}

var _ RWLocker = &NoLock{}

func (r *NoLock) Lock() {
	if r.locked {
		panic("already locked")
	}
	if r.rlocks != 0 {
		panic("rlocks not zero")
	}
	r.locked = true
}

func (r *NoLock) Unlock() {
	if !r.locked {
		panic("not locked")
	}
	r.locked = false
}

func (r *NoLock) RLock() {
	if r.locked {
		panic("already locked")
	}
	r.rlocks++
}

func (r *NoLock) RUnlock() {
	if r.rlocks <= 0 {
		panic("not rlocked")
	}
	r.rlocks--
}
