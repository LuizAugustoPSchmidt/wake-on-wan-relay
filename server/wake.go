package main

import (
	"log"
	"net/http"
)

func (s *ServerApp) WakeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received /wake call")

	if !s.Authenticate(r) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := s.sendPacket(s.targetMAC); err != nil {
		http.Error(w, "WOL broadcast failed", http.StatusInternalServerError)
		return
	}
	w.Write([]byte("Authenticated. Magic packet broadcasted locally.\n"))
}
