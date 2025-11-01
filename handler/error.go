package handler

import (
	"encoding/json"
	"net/http"
)

func writeInternalError(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	merr := HttpResult{
		Code:  "system_error",
		Error: err.Error(),
	}
	json.NewEncoder(w).Encode(merr)
}

func writeForbiden(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
	merr := HttpResult{
		Code: "forbiden",
	}
	json.NewEncoder(w).Encode(merr)
}
