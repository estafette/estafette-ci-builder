package builder

import "testing"

func TestRLock(t *testing.T) {
	t.Run("ReadLockDoesNotGetBlockedByReadLockOnSameKey", func(t *testing.T) {

		mapMutex := NewMapMutex()
		mapMutex.RLock("key1")
		defer mapMutex.RUnlock("key1")

		// act
		mapMutex.RLock("key1")
		defer mapMutex.RUnlock("key1")
	})

	t.Run("ReadLockDoesNotGetBlockedByWriteLockOnOtherKey", func(t *testing.T) {

		mapMutex := NewMapMutex()
		mapMutex.Lock("key2")
		defer mapMutex.Unlock("key2")

		// act
		mapMutex.RLock("key1")
		defer mapMutex.RUnlock("key1")
	})
}
