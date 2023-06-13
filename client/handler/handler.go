package handler

import "github.com/gorilla/mux"

type Handler struct {
}

func NewHandler() *Handler {
	return &Handler{}
}

// NewRouter returns a new router object for the specified routes.
// It also wraps each request in an authentication method and logging method.
func NewRouter(h *Handler) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range GetRoutes(h) {
		h := route.HandlerFunc
		router.Methods(route.Method).Path(route.Pattern).Name(route.Name).Handler(h)
	}

	return router
}
