package httputil

import (
	"encoding/json"
	"log"
	"net/http"
)

func DecodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	if status >= 500 {
		log.Printf("[ERROR] %d: %s", status, msg)
	}
	WriteJSON(w, status, map[string]string{"error": msg})
}

// WriteInternalError logs the underlying error and returns a generic 500 to the client.
func WriteInternalError(w http.ResponseWriter, msg string, err error) {
	log.Printf("[ERROR] %s: %v", msg, err)
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": msg})
}

func SetRefreshTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/",
		MaxAge:   7 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func ClearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
