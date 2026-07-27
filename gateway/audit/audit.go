dingdong
package audit

import (
	"database/sql"
	"encoding/json"
	"log"
)

// LogEntry represents an audit event.
type LogEntry struct {
	AgentID         string
	RequestPayload  interface{}
	ResponsePayload interface{}
	Cost            float64
	Status          string
}

// LogCh is the buffered channel for async DB inserts.
var LogCh = make(chan LogEntry, 1000)

// spin up a goroutine to listen for audit logs and then insert them into PG
func StartWorker(db *sql.DB) {
	go func() {
		for entry := range LogCh {
			reqJSON, errReq := json.Marshal(entry.RequestPayload)
			if errReq != nil {
				reqJSON = []byte(`{"error": "unmarshalable request"}`)
			}
			respJSON, errResp := json.Marshal(entry.ResponsePayload)
			if errResp != nil {
				respJSON = []byte(`{"error": "unmarshalable response"}`)
			}
			_, err := db.Exec(`
				INSERT INTO audit_logs (agent_id, request_payload, response_payload, cost, status)
				VALUES ($1, $2, $3, $4, $5)`,
				entry.AgentID, reqJSON, respJSON, entry.Cost, entry.Status)
			if err != nil {
				log.Printf("Failed to insert audit log: %v", err)
			}
		}
	}()
}
