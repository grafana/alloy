package util

import (
	"errors"
	"sync"
)

func ErrorsJoinConcurrent(errs *error, err error, mutex *sync.Mutex) {
	mutex.Lock()
	*errs = errors.Join(*errs, err)
	mutex.Unlock()
}
