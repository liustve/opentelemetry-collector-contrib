// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"

// httpAttribute preserves existing stable-only HTTP support. When both
// conventions are present, the migration gate determines which one takes
// precedence. Disabling legacy conventions also disables the fallback.
func httpAttribute[T any](lookup func(string) (T, bool), stableKey, legacyKey string) (T, bool) {
	if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if value, ok := lookup(stableKey); ok {
			return value, true
		}
	}
	if !metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if value, ok := lookup(legacyKey); ok {
			return value, true
		}
	}
	return lookup(stableKey)
}
