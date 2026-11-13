package providers

type volumeTargets struct {
	byKey map[string]*volumeTarget
	seq   uint64
}

type volumeTarget struct {
	percent int
	dest    string
	seq     uint64
	sent    uint64
	busy    bool
}

func newVolumeTargets() *volumeTargets {
	return &volumeTargets{byKey: map[string]*volumeTarget{}}
}

func (v *volumeTargets) pending() bool {
	return len(v.byKey) > 0
}

func (v *volumeTargets) percent(key string) (int, bool) {
	t, ok := v.byKey[key]
	if !ok {
		return 0, false
	}
	return t.percent, true
}

func (v *volumeTargets) dest(key string) (string, bool) {
	t, ok := v.byKey[key]
	if !ok {
		return "", false
	}
	return t.dest, true
}

func (v *volumeTargets) set(key, dest string, percent int) bool {
	t, ok := v.byKey[key]
	if !ok {
		t = &volumeTarget{}
		v.byKey[key] = t
	}
	v.seq++
	t.percent = percent
	t.dest = dest
	t.seq = v.seq
	return !t.busy
}

func (v *volumeTargets) next(key string) (percent int, dest string, seq uint64) {
	t := v.byKey[key]
	t.busy = true
	t.sent = t.seq
	return t.percent, t.dest, t.seq
}

func (v *volumeTargets) done(key string, seq uint64) bool {
	t, ok := v.byKey[key]
	if !ok || seq != t.sent {
		return false
	}
	t.busy = false
	if t.seq != t.sent {
		return true
	}
	delete(v.byKey, key)
	return false
}

func (v *volumeTargets) drop(key string) {
	delete(v.byKey, key)
}

func (v *volumeTargets) requeue() []string {
	keys := make([]string, 0, len(v.byKey))
	for key, t := range v.byKey {
		t.busy = false
		keys = append(keys, key)
	}
	return keys
}
