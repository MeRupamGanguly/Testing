package config

import "time"

type LaunchDarklyProperties struct {
	SdkKey      string
	OfflineMode bool
}

type FeatureFlagCacheProperties struct {
	CacheEnabled           bool
	ExpireAfterWriteMinute time.Duration
}
