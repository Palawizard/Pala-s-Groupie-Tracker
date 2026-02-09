package handlers

import "palasgroupietracker/internal/store"

var appStore *store.Store

// SetStore wires the shared database store into handlers
func SetStore(s *store.Store) {
	appStore = s
}
