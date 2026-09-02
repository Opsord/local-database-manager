package core

import (
	"encoding/json"
	"os"
	"time"
)

// AgentDebugLog writes one NDJSON debug line for the active debug session.
func AgentDebugLog(hypothesisID, location, message string, data map[string]any) {
	// #region agent log
	payload := map[string]any{
		"sessionId":    "df70f9",
		"timestamp":    time.Now().UnixMilli(),
		"hypothesisId": hypothesisID,
		"location":     location,
		"message":      message,
		"data":         data,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	f, err := os.OpenFile(`C:\Users\andre\Programacion\Personal\local-database-manager\debug-df70f9.log`, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
	// #endregion
}
