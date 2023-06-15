package handler

import (
	"context"
	"github.com/flybits/gophercon2023/server/pb"
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
		err := h.ServiceManager.GetStreamFromServer(context.Background(), &pb.DataRequest{})
		if err != nil {
			log.Printf("error happened: %v", err)
		}
	}()
	w.Header().Set("Content-Type", "application/text; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("stream started"))
	return
}
