package http

import (
	"encoding/json"
	"net/http"
)

func jsonNewEncoder(w http.ResponseWriter) *json.Encoder {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	return encoder
}
