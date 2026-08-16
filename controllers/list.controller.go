package controllers

import "net/http"

// Esto hay que transformalo a TCP para hacerlo un socket
func ListOportunitiesController() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Solo peticiones POST entran aquí"))
	}
}