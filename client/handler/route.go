package handler

import (
	"context"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
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
		err := h.controller.PerformStreaming(context.Background(), 0, uuid.New().String())
		if err != nil {
			log.Printf("error happened: %v", err)
		}
	}()
	w.Header().Set("Content-Type", "application/text; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	podName := os.Getenv("CONFIG_POD_NAME")
	w.Write([]byte("stream started on pod " + podName))
	return
}

func (h *Handler) OOM(w http.ResponseWriter, r *http.Request) {

	// causing intentional OOM
	size := 10000000000000
	b := make([]int64, size)
	for i := 0; i < size; i++ {
		b[i] = 12233445566677777
	}
	return
}
