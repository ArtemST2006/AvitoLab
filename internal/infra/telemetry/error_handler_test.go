package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenTelemetryErrorUsesJSONLogger(t *testing.T) {
	var output bytes.Buffer
	logger := newLogger("info", "test", &output)
	slogErrorHandler{log: logger}.Handle(errors.New("collector unavailable"))

	var record map[string]any
	require.NoError(t, json.Unmarshal(output.Bytes(), &record))
	require.Equal(t, "ERROR", record["level"])
	require.Equal(t, "OpenTelemetry export failed", record["msg"])
	require.Equal(t, "collector unavailable", record["error"])
}
