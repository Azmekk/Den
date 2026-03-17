package httputil

import (
	"encoding/json"
	"log"
	"net/http"
)

func DecodeJSON(request *http.Request, target any) error {
	defer request.Body.Close()
	return json.NewDecoder(request.Body).Decode(target)
}

func WriteJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	json.NewEncoder(writer).Encode(value)
}

func WriteError(writer http.ResponseWriter, status int, msg string) {
	if status >= 500 {
		log.Printf("[ERROR] %d: %s", status, msg)
	}
	WriteJSON(writer, status, map[string]string{"error": msg})
}

// WriteInternalError logs the underlying error and returns a generic 500 to the client.
func WriteInternalError(writer http.ResponseWriter, msg string, err error) {
	log.Printf("[ERROR] %s: %v", msg, err)
	WriteJSON(writer, http.StatusInternalServerError, map[string]string{"error": msg})
}
