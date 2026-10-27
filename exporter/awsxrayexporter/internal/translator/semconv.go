// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
)

// httpAttribute preserves existing stable-only HTTP support. When both
// conventions are present, the migration gate determines which one takes
// precedence. Disabling legacy conventions also disables the fallback.
func httpAttribute(attributes pcommon.Map, stableKey, legacyKey string) (pcommon.Value, bool) {
	if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if value, ok := attributes.Get(stableKey); ok {
			return value, true
		}
	}
	if !metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if value, ok := attributes.Get(legacyKey); ok {
			return value, true
		}
	}
	return attributes.Get(stableKey)
}
