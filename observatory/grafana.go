package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

const grafanaURL = "http://localhost:3000"

func postGrafanaAnnotation(text, tags string) {
	body, _ := json.Marshal(map[string]any{
		"time": time.Now().UnixMilli(),
		"text": text,
		"tags": []string{"observatory", tags},
	})
	req, err := http.NewRequest(http.MethodPost, grafanaURL+"/api/annotations", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (srv *Server) annotateStep(desc string) {
	go postGrafanaAnnotation(desc, "observatory")
}
