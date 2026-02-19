package core

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"

	"go.uber.org/zap"
)

// JoinMapKeys joins the keys of a map into a comma-separated string.
// Useful for error messages that need to list valid values.
func JoinMapKeys[T comparable](m map[T]struct{}) string {
	keys := slices.Collect(maps.Keys(m))
	sliceStrings := make([]string, len(keys))
	for i, k := range keys {
		sliceStrings[i] = fmt.Sprintf("%v", k)
	}
	return strings.Join(sliceStrings, ", ")
}

func WriteJSONResponse(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		zap.L().Error("Failed to encode JSON response", zap.Error(err))
	}
}
