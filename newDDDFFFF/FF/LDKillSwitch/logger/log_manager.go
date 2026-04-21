package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"featuresgflags/LDKillSwitch/service"

	"github.com/launchdarkly/go-sdk-common/v3/ldcontext"
)

var internalLog = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

type LogLevelManager struct {
	flagSvc      *service.FeatureFlagService
	flagKey      string
	ctx          ldcontext.Context
	pollInterval time.Duration
	level        atomic.Value
	stopCh       chan struct{}
}

func NewLogLevelManager(
	flagSvc *service.FeatureFlagService,
	flagKey string,
	ctx ldcontext.Context,
	pollInterval time.Duration,
) *LogLevelManager {
	mgr := &LogLevelManager{
		flagSvc:      flagSvc,
		flagKey:      flagKey,
		ctx:          ctx,
		pollInterval: pollInterval,
		stopCh:       make(chan struct{}),
	}
	mgr.level.Store(slog.LevelInfo)
	return mgr
}

func (m *LogLevelManager) Start() {
	go m.pollLoop()
}

func (m *LogLevelManager) Stop() {
	close(m.stopCh)
}

func (m *LogLevelManager) Level() slog.Level {
	return m.level.Load().(slog.Level)
}

func (m *LogLevelManager) pollLoop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	// Initial fetch
	m.refreshLevel()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.refreshLevel()
		}
	}
}

func (m *LogLevelManager) refreshLevel() {
	defaultValue := []interface{}{"ERROR"}
	// Use no-cache to get the absolute latest value from LaunchDarkly
	raw := m.flagSvc.GetJSONFlagNoCache(m.flagKey, m.ctx, defaultValue)

	var levelsSlice []string
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				levelsSlice = append(levelsSlice, str)
			}
		}
	case []string:
		levelsSlice = v
	case json.RawMessage:
		var strSlice []string
		if err := json.Unmarshal(v, &strSlice); err == nil {
			levelsSlice = strSlice
		} else {
			var ifaceSlice []interface{}
			if err := json.Unmarshal(v, &ifaceSlice); err == nil {
				for _, item := range ifaceSlice {
					if str, ok := item.(string); ok {
						levelsSlice = append(levelsSlice, str)
					}
				}
			} else {
				internalLog.Error("cannot unmarshal log level flag", "value", string(v))
				return
			}
		}
	default:
		internalLog.Error("unexpected type for log level flag", "type", fmt.Sprintf("%T", raw))
		return
	}

	if len(levelsSlice) == 0 {
		internalLog.Warn("empty log level slice")
		return
	}

	var minLevel slog.Level = slog.LevelError
	for _, str := range levelsSlice {
		var level slog.Level
		switch str {
		case "DEBUG":
			level = slog.LevelDebug
		case "INFO":
			level = slog.LevelInfo
		case "WARN":
			level = slog.LevelWarn
		case "ERROR":
			level = slog.LevelError
		default:
			continue
		}
		if level < minLevel {
			minLevel = level
		}
	}

	old := m.level.Swap(minLevel).(slog.Level)
	if old != minLevel {
		internalLog.Info("Log level changed", "from", old.String(), "to", minLevel.String())
	}
}

func (m *LogLevelManager) SetLogLevelHandler(baseHandler slog.Handler) slog.Handler {
	return &dynamicLevelHandler{
		Handler: baseHandler,
		levelFn: m.Level,
	}
}

type dynamicLevelHandler struct {
	slog.Handler
	levelFn func() slog.Level
}

func (h *dynamicLevelHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.levelFn()
}
