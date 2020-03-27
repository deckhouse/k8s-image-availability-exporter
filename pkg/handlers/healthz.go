package handlers

import (
	"net/http"
	"sync"
)

var (
	mutex   sync.RWMutex
	healthy = false
)

func UpdateHealth(isHealthy bool) {
	mutex.Lock()
	healthy = isHealthy
	mutex.Unlock()
}

func Healthz(w http.ResponseWriter, _ *http.Request) {
	mutex.RLock()
	isHealthy := healthy
	mutex.RUnlock()
	if isHealthy {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Unhealthy"))
	}
}
