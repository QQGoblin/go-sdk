/*
Copyright 2019 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lock

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"sync"
)

type keyMutex struct {
	mux   sync.Mutex
	locks sets.String
}

func NewKeyMutex() keyMutex {
	return keyMutex{
		locks: sets.NewString(),
	}
}

func (mm keyMutex) LockKey(key string) bool {
	mm.mux.Lock()
	defer mm.mux.Unlock()
	if mm.locks.Has(key) {
		return false
	}
	mm.locks.Insert(key)
	return true
}

func (mm *keyMutex) UnlockKey(key string) {
	mm.mux.Lock()
	defer mm.mux.Unlock()
	mm.locks.Delete(key)
}
