package handler

import (
	"context"
	"log"
	"net/http"
)

// Route is the structure for an http route
type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

func (h *Handler) GetRoutes() []Route {
	return []Route{
		{
			Name:        "CheckHealth",
			Method:      "GET",
			Pattern:     "/client/health",
			HandlerFunc: h.CheckHealth,
		},
		{
			Name:        "Start",
			Method:      "GET",
			Pattern:     "/start",
			HandlerFunc: h.Start,
		},
		{
			Name:        "OOM",
			Method:      "GET",
			Pattern:     "/oom",
			HandlerFunc: h.OOM,
		},
	}

}

func (h *Handler) CheckHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/text; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
	return
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {

	go func() {
		err := h.ServiceManager.GetStreamFromServer(context.Background(), 0)
		if err != nil {
			log.Printf("error happened: %v", err)
		}
	}()
	w.Header().Set("Content-Type", "application/text; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("stream started"))
	return
}

func (h *Handler) OOM(w http.ResponseWriter, r *http.Request) {

	// causing intentional OOM
	size := 10000000000000
	bubu := make([]int64, size)
	for i := 0; i < size; i++ {
		bubu[i] = 12233445566677777
	}
	w.Header().Set("Content-Type", "application/text; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("stream started"))
	return
}
