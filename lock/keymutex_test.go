package lock

import "testing"

func TestKeyMutex(t *testing.T) {
	t.Parallel()
	fakeID := "fake-id"
	locks := NewKeyMutex()

	ok := locks.LockKey(fakeID)

	if !ok {
		t.Errorf("TryAcquire failed: want (%v), got (%v)",
			true, ok)
	}

	ok = locks.LockKey(fakeID)

	if ok {
		t.Errorf("TryAcquire failed: want (%v), got (%v)",
			false, ok)
	}

	locks.UnlockKey(fakeID)
	ok = locks.LockKey(fakeID)

	if !ok {
		t.Errorf("TryAcquire failed: want (%v), got (%v)",
			true, ok)
	}
}
