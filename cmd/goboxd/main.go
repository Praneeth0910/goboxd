package main

import (
	"net/http"
	"fmt"
	"log"
	"encoding/json"
	"os"
	"crypto/rand"
	"os/exec"
	"context"
	"time"
)

var languageMap = map[string]string{
    "py": "python3",
    "sh": "bash",
    // We can easily add "js": "node", "cpp": "g++", etc., later
}

func main() {
	http.HandleFunc("/healthz", HandleHealthz)
	http.HandleFunc("/run", HandleRun)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func HandleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w,`{"status":"ok"}`)
}

type RunRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Extention string `json:"extention"`
}
type ExecutionResponse struct {
	Status        string `json:"status"`
	Output        string `json:"output,omitempty"`
	ViolationType string `json:"violation_type,omitempty"`
	Action        string `json:"action,omitempty"`
	Message       string `json:"message,omitempty"`
}
func HandleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST"{
		http.Error(w,"Method not allowed",405)
		return
	}
	var req RunRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
    http.Error(w, "Invalid JSON payload", 400)
    return
	}
	unique_fileID := make([]byte, 4)
	rand.Read(unique_fileID)
	filename := fmt.Sprintf("/tmp/code_%x.%s", unique_fileID, req.Extention)
	err = os.WriteFile(filename, []byte(req.Code), 0644)
	if err != nil {
		http.Error(w, "Failed to write code to file", 500)
		return
	}
	//Just to prove it works, let's print it to your Ubuntu terminal:
	fmt.Printf("Received code to run in %s:\n%s\n and filename: %s", req.Language, req.Code, filename)
	//Set the time bomb BEFORE running anything
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel() 

    //Bind the OS command strictly to the time bomb
    cmd := exec.CommandContext(ctx, languageMap[req.Language], filename)

    //Run the command. If it takes > 2s, the context kills it violently.
    output, err := cmd.CombinedOutput()
    
    //Format the response
	w.Header().Set("Content-Type", "application/json")

	if err != nil {
		// INCIDENT: The time bomb killed the process
		if ctx.Err() == context.DeadlineExceeded {
			w.WriteHeader(http.StatusRequestTimeout) // HTTP 408
			json.NewEncoder(w).Encode(ExecutionResponse{
				Status:        "incident",
				ViolationType: "Timeout Exceeded",
				Action:        "SIGKILL",
			})
			return
		}

		// ERROR: Standard code crash (e.g., Syntax Error)
		w.WriteHeader(http.StatusBadRequest) // HTTP 400
		json.NewEncoder(w).Encode(ExecutionResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	//Send the captured output
	w.WriteHeader(http.StatusOK) // HTTP 200
	json.NewEncoder(w).Encode(ExecutionResponse{
		Status: "success",
		Output: string(output),
	})
}