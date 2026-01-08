package common

import (
	"context"
	"net/http"
)

type contextKey string

const key contextKey = "requestor"

func GetRequestor(r *http.Request) string {
	return r.Context().Value(key).(string)
}

func GetRequestWithRequestor(r *http.Request, username string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), key, username))
}
