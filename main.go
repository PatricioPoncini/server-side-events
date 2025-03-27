package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/v4/mem"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

func colorLog(level, message string) {
	var color string
	switch level {
	case "INFO":
		color = colorCyan
	case "WARN":
		color = colorYellow
	case "ERROR":
		color = colorRed
	default:
		color = colorReset
	}
	log.Printf("%s[%s] %s%s", color, level, message, colorReset)
}

func main() {
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)

	http.HandleFunc("/events", sseHandler)

	http.HandleFunc("/ping", pingHandler)

	colorLog("INFO", "Server started and listening on port :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		colorLog("ERROR", fmt.Sprintf("unable to start server: %s", err.Error()))
	}
}

func WriteJSONResponse(w http.ResponseWriter, status int, v any) error {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(v)
}

func pingHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]any{
		"message": "pong",
	}
	WriteJSONResponse(w, http.StatusOK, response)
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	memT := time.NewTicker(time.Second)
	defer memT.Stop()

	cpuT := time.NewTicker(time.Second)
	defer cpuT.Stop()

	clientGone := r.Context().Done()

	rc := http.NewResponseController(w)

	for {
		select {
		case <-clientGone:
			colorLog("INFO", fmt.Sprintf("client has disconnected"))
			return
		case <-memT.C:
			m, err := mem.VirtualMemory()
			if err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to get mem: %s", err.Error()))
				return
			}
			if _, err := fmt.Fprintf(w, "event:memTotal\ndata:%d\n\n", m.Total); err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to write memTotal: %s", err.Error()))
				return
			}
			if _, err := fmt.Fprintf(w, "event:memUsed\ndata:%d\n\n", m.Used); err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to write memUsed: %s", err.Error()))
				return
			}
			if _, err := fmt.Fprintf(w, "event:memPerc\ndata:%.2f%%\n\n", m.UsedPercent); err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to write memPerc: %s", err.Error()))
				return
			}
			rc.Flush()
		case <-cpuT.C:
			c, err := cpu.Times(false)
			if err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to get cpu: %s", err.Error()))
				return
			}
			if _, err := fmt.Fprintf(w, "event:cpuUser\ndata:%.2f\n\n", c[0].User); err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to write cpuUser: %s", err.Error()))
				return
			}
			if _, err := fmt.Fprintf(w, "event:cpuSys\ndata:%.2f\n\n", c[0].System); err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to write cpuSys: %s", err.Error()))
				return
			}
			if _, err := fmt.Fprintf(w, "event:cpuIdle\ndata:%.2f\n\n", c[0].Idle); err != nil {
				colorLog("ERROR", fmt.Sprintf("unable to write cpuIdle: %s", err.Error()))
				return
			}
			rc.Flush()
		}
	}
}
