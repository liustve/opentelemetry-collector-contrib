// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
)

func enableGate(t *testing.T, gate *featuregate.Gate) {
	prev := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), prev))
	})
}

func TestClientSpanWithNewAndLegacyDBAttributesPrefersNew(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate)

	attributes := make(map[string]pcommon.Value)
	attributes["db.system.name"] = pcommon.NewValueStr("postgresql")
	attributes["db.namespace"] = pcommon.NewValueStr("orders")
	attributes["db.query.text"] = pcommon.NewValueStr("SELECT 1")
	attributes["user.name"] = pcommon.NewValueStr("new_user")
	attributes["db.system"] = pcommon.NewValueStr("mysql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	attributes["db.statement"] = pcommon.NewValueStr("SELECT 2")
	attributes["db.user"] = pcommon.NewValueStr("legacy_user")
	span := constructSQLSpan(attributes)

	// Map iteration order is random; repeat to make an order-dependent bug surface.
	for range 50 {
		_, sqlData := makeSQL(span, attributes)
		require.NotNil(t, sqlData)
		assert.Equal(t, "postgresql", *sqlData.DatabaseType)
		assert.Nil(t, sqlData.ConnectionString)
		assert.Equal(t, "SELECT 1", *sqlData.SanitizedQuery)
		assert.Equal(t, "legacy_user", *sqlData.User)
	}
}

func TestClientSpanWithLegacyDBAttributesOnlyWithEmitV1(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate)

	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("mysql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	span := constructSQLSpan(attributes)

	_, sqlData := makeSQL(span, attributes)
	require.NotNil(t, sqlData)
	assert.Equal(t, "mysql", *sqlData.DatabaseType)
	assert.Equal(t, "localhost/customers", *sqlData.ConnectionString)
}

func TestClientSpanWithStatementAttribute(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("mysql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	attributes["db.statement"] = pcommon.NewValueStr("SELECT * FROM user WHERE user_id = ?")
	attributes["db.user"] = pcommon.NewValueStr("readonly_user")
	attributes["db.connection_string"] = pcommon.NewValueStr("mysql://db.example.com:3306")
	attributes["net.peer.name"] = pcommon.NewValueStr("db.example.com")
	attributes["net.peer.port"] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)

	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(sqlData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "mysql://db.example.com:3306/customers")
}

func TestClientSpanWithNonSQLDatabase(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("redis")
	attributes["db.name"] = pcommon.NewValueStr("0")
	attributes["db.statement"] = pcommon.NewValueStr("SET key value")
	attributes["db.user"] = pcommon.NewValueStr("readonly_user")
	attributes["db.connection_string"] = pcommon.NewValueStr("redis://db.example.com:3306")
	attributes["net.peer.name"] = pcommon.NewValueStr("db.example.com")
	attributes["net.peer.port"] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)
	assert.Nil(t, sqlData)
	assert.NotNil(t, filtered)
}

func TestClientSpanWithoutDBurlAttribute(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	attributes["db.system"] = pcommon.NewValueStr("postgresql")
	attributes["db.name"] = pcommon.NewValueStr("customers")
	attributes["db.statement"] = pcommon.NewValueStr("SELECT * FROM user WHERE user_id = ?")
	attributes["db.user"] = pcommon.NewValueStr("readonly_user")
	attributes["db.connection_string"] = pcommon.NewValueStr("")
	attributes["net.peer.name"] = pcommon.NewValueStr("db.example.com")
	attributes["net.peer.port"] = pcommon.NewValueStr("3306")
	span := constructSQLSpan(attributes)

	filtered, sqlData := makeSQL(span, attributes)
	assert.NotNil(t, filtered)
	assert.NotNil(t, sqlData)

	assert.Equal(t, "users.findUnique", *sqlData.URL)
}

func constructSQLSpan(attributes map[string]pcommon.Value) ptrace.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)

	// constructSpanAttributes() in segment_test accepts a map of interfaces...
	interfaceAttributes := make(map[string]any)
	for k, v := range attributes {
		interfaceAttributes[k] = v
	}
	spanAttributes := constructSpanAttributes(interfaceAttributes)

	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("users.findUnique")
	span.SetKind(ptrace.SpanKindClient)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(0)
	status.SetMessage("OK")
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func TestMakeSQLStableEndpointAndNoUserNameMapping(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate)
	for _, host := range []string{"db.example.com", "2001:db8::1"} {
		span := constructHTTPClientSpan(map[string]any{
			"db.system.name": "postgresql",
			"db.namespace":   "orders",
			"db.query.text":  "SELECT $1",
			"server.address": host,
			"server.port":    int64(5432),
			"user.name":      "application-user",
		})
		attributes, _ := makeHTTP(span)
		filtered, sqlData := makeSQL(span, attributes)
		require.NotNil(t, sqlData)
		if host == "db.example.com" {
			assert.Equal(t, "db.example.com:5432/orders", *sqlData.ConnectionString)
		} else {
			assert.Equal(t, "[2001:db8::1]:5432/orders", *sqlData.ConnectionString)
		}
		assert.Nil(t, sqlData.User)
		require.Contains(t, filtered, "user.name")
		require.Contains(t, filtered, "server.address")
		assert.Equal(t, "application-user", filtered["user.name"].Str())
		assert.Equal(t, host, filtered["server.address"].Str())
	}
}

func TestMakeSQLStableDBWithoutEndpointOmitsConnectionString(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate)
	span := constructHTTPClientSpan(map[string]any{
		"db.system.name": "postgresql",
		"db.namespace":   "orders",
	})
	attributes, _ := makeHTTP(span)
	_, sqlData := makeSQL(span, attributes)
	require.NotNil(t, sqlData)
	assert.Nil(t, sqlData.ConnectionString)
}

func TestMakeSQLMixedConventionsRespectFeatureGates(t *testing.T) {
	for _, mode := range []string{"default", "dual", "stable"} {
		t.Run(mode, func(t *testing.T) {
			if mode != "default" {
				enableGate(t, metadata.ExporterAwsxrayEmitV1DBConventionsFeatureGate)
			}
			if mode == "stable" {
				enableGate(t, metadata.ExporterAwsxrayDontEmitV0DBConventionsFeatureGate)
			}
			span := constructHTTPClientSpan(map[string]any{
				"db.system":            "mysql",
				"db.name":              "legacy-db",
				"db.statement":         "SELECT 2",
				"db.connection_string": "mysql://legacy.example.com:3306",
				"db.user":              "db-user",
				"db.system.name":       "postgresql",
				"db.namespace":         "orders",
				"db.query.text":        "SELECT $1",
				"db.query.summary":     "SELECT orders",
				"server.address":       "db.example.com",
				"server.port":          int64(5432),
				"user.name":            "application-user",
			})
			attributes, _ := makeHTTP(span)
			filtered, sqlData := makeSQL(span, attributes)
			require.NotNil(t, sqlData)
			require.Contains(t, filtered, "db.query.summary")
			require.Contains(t, filtered, "user.name")
			if mode == "default" {
				assert.Equal(t, "mysql", *sqlData.DatabaseType)
				assert.Equal(t, "SELECT 2", *sqlData.SanitizedQuery)
				assert.Equal(t, "mysql://legacy.example.com:3306/legacy-db", *sqlData.ConnectionString)
			} else {
				assert.Equal(t, "postgresql", *sqlData.DatabaseType)
				assert.Equal(t, "SELECT $1", *sqlData.SanitizedQuery)
				if mode == "dual" {
					assert.Equal(t, "mysql://legacy.example.com:3306/orders", *sqlData.ConnectionString)
				} else {
					assert.Equal(t, "db.example.com:5432/orders", *sqlData.ConnectionString)
				}
			}
			if mode == "stable" {
				assert.Nil(t, sqlData.User)
			} else {
				assert.Equal(t, "db-user", *sqlData.User)
			}
		})
	}
}
