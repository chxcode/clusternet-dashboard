package server

import (
	"context"
	"net/http"

	"github.com/chxcode/clusternet-dashboard/internal/kube"
)

func catalogHandler[T any](explorer *kube.Explorer, key string, list func(*kube.Explorer, context.Context) ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items := []T{}
		if explorer != nil {
			var err error
			items, err = list(explorer, r.Context())
			if err != nil {
				writeError(w, err)
				return
			}
		}
		writeJSON(map[string][]T{key: items})(w, r)
	}
}
