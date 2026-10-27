// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
)

func TestClientSpanWithURLAttribute(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.url"] = "https://api.example.com/users/junit"
	attributes["http.response.status_code"] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestClientSpanWithURLAttributeStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["url.full"] = "https://api.example.com/users/junit"
	attributes["http.response.status_code"] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestClientSpanWithSchemeHostTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "https"
	attributes["http.host"] = "api.example.com"
	attributes["http.target"] = "/users/junit"
	attributes["http.response.status_code"] = 200
	attributes["user.id"] = "junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestClientSpanWithSchemeHostTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = "GET"
	attributes["url.scheme"] = "https"
	attributes["http.host"] = "api.example.com"
	attributes["url.path"] = "/users/junit"
	attributes["url.query"] = "v=1"
	attributes["http.response.status_code"] = 200
	attributes["user.id"] = "junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit?v=1")
}

func TestClientSpanWithPeerAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "http"
	attributes["net.peer.name"] = "kb234.example.com"
	attributes["net.peer.port"] = 8080
	attributes["net.peer.ip"] = "10.8.17.36"
	attributes["http.target"] = "/users/junit"
	attributes["http.response.status_code"] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	assert.NotNil(t, httpData.Request.URL)

	assert.Equal(t, "10.8.17.36", *httpData.Request.ClientIP)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/users/junit")
}

func TestClientSpanWithPeerAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["url.scheme"] = "http"
	attributes["net.peer.name"] = "kb234.example.com"
	attributes["net.peer.port"] = 8080
	attributes["net.peer.ip"] = "10.8.17.36"
	attributes["url.query"] = "users=junit"
	attributes["http.response.status_code"] = 200
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "10.8.17.36", *httpData.Request.ClientIP)

	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/?users=junit")
}

func TestClientSpanServerPortPreferredWithEmitV1(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate)

	tests := []struct {
		name       string
		attributes map[string]any
		want       string
	}{
		{
			name: "server.* win over net.peer.*",
			attributes: map[string]any{
				"server.address": "new.example.com", "server.port": 9090,
				"net.peer.name": "old.example.com", "net.peer.port": 8080,
			},
			want: "http://new.example.com:9090/users/junit",
		},
		{
			name:       "net.peer.port fallback when server.port absent",
			attributes: map[string]any{"server.address": "new.example.com", "net.peer.port": 8080},
			want:       "http://new.example.com:8080/users/junit",
		},
		{
			name:       "server.port with legacy host",
			attributes: map[string]any{"net.peer.name": "old.example.com", "server.port": 9090, "net.peer.port": 8080},
			want:       "http://old.example.com:9090/users/junit",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.attributes["http.request.method"] = http.MethodGet
			tt.attributes["url.path"] = "/users/junit"
			_, httpData := makeHTTP(constructHTTPClientSpan(tt.attributes))
			require.NotNil(t, httpData)
			require.NotNil(t, httpData.Request.URL)
			assert.Equal(t, tt.want, *httpData.Request.URL)
		})
	}
}

func TestClientSpanWithHttpPeerAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.client_ip"] = "1.2.3.4"
	attributes["net.peer.ip"] = "10.8.17.36"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "1.2.3.4", *httpData.Request.ClientIP)
}

func TestClientSpanWithHttpPeerAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["url.full"] = "https://api.example.com/users/junit"
	attributes["client.address"] = "1.2.3.4"
	attributes["network.peer.address"] = "10.8.17.36"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)

	assert.Equal(t, "1.2.3.4", *httpData.Request.ClientIP)
}

func TestClientSpanWithPeerIp4Attributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "http"
	attributes["net.peer.ip"] = "10.8.17.36"
	attributes["net.peer.port"] = "8080"
	attributes["http.target"] = "/users/junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)
	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://10.8.17.36:8080/users/junit")
}

func TestClientSpanWithPeerIp6Attributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "https"
	attributes["net.peer.ip"] = "2001:db8:85a3::8a2e:370:7334"
	attributes["net.peer.port"] = "443"
	attributes["http.target"] = "/users/junit"
	span := constructHTTPClientSpan(attributes)

	filtered, httpData := makeHTTP(span)
	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://2001:db8:85a3::8a2e:370:7334/users/junit")
}

func TestServerSpanWithURLAttribute(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.url"] = "https://api.example.com/users/junit"
	attributes["http.client_ip"] = "192.168.15.32"
	attributes["http.user_agent"] = "PostmanRuntime/7.21.0"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithURLAttributeStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["url.full"] = "https://api.example.com/users/junit"
	attributes["client.address"] = "192.168.15.32"
	attributes["user_agent.original"] = "PostmanRuntime/7.21.0"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeHostTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "https"
	attributes["http.host"] = "api.example.com"
	attributes["http.target"] = "/users/junit"
	attributes["http.client_ip"] = "192.168.15.32"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeHostTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["url.scheme"] = "https"
	attributes["server.address"] = "api.example.com"
	attributes["url.path"] = "/users/junit"
	attributes["client.address"] = "192.168.15.32"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithNewConventionsWithURLPath(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["server.address"] = "localhost"
	attributes["url.scheme"] = "http"
	attributes["url.query"] = "?version=test"
	attributes["url.path"] = "/api"
	attributes["client.address"] = "127.0.0.1"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://localhost/api?version=test")
}

func TestServerSpanWithSchemeServernamePortTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "https"
	attributes["http.server_name"] = "api.example.com"
	attributes["net.host.port"] = 443
	attributes["http.target"] = "/users/junit"
	attributes["http.client_ip"] = "192.168.15.32"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/users/junit")
}

func TestServerSpanWithSchemeServernamePortTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["url.scheme"] = "https"
	attributes["server.address"] = "api.example.com"
	attributes["server.port"] = 443
	attributes["url.query"] = "users=junit"
	attributes["client.address"] = "192.168.15.32"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "https://api.example.com/?users=junit")
}

func TestServerSpanWithSchemeNamePortTargetAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "http"
	attributes["host.name"] = "kb234.example.com"
	attributes["net.host.port"] = 8080
	attributes["http.target"] = "/users/junit"
	attributes["http.client_ip"] = "192.168.15.32"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/users/junit")
}

func TestServerSpanWithSchemeNamePortTargetAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["url.scheme"] = "http"
	attributes["server.address"] = "kb234.example.com"
	attributes["server.port"] = 8080
	attributes["url.path"] = "/users/junit"
	attributes["client.address"] = "192.168.15.32"
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.NotNil(t, httpData)
	assert.NotNil(t, filtered)
	w := testWriters.borrow()
	require.NoError(t, w.Encode(httpData))
	jsonStr := w.String()
	testWriters.release(w)
	assert.Contains(t, jsonStr, "http://kb234.example.com:8080/users/junit")
}

func TestSpanWithNotEnoughHTTPRequestURLAttributes(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.scheme"] = "http"
	attributes["http.client_ip"] = "192.168.15.32"
	attributes["http.user_agent"] = "PostmanRuntime/7.21.0"
	attributes["http.target"] = "/users/junit"
	attributes["net.host.port"] = 443
	attributes["net.peer.port"] = 8080
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.URL)
	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Equal(t, http.MethodGet, *httpData.Request.Method)
	assert.Equal(t, "PostmanRuntime/7.21.0", *httpData.Request.UserAgent)
	contentLength := *httpData.Response.ContentLength.(*int64)
	assert.Equal(t, int64(12452), contentLength)
	assert.Equal(t, int64(200), *httpData.Response.Status)
	assert.NotNil(t, filtered)
}

func TestSpanWithNotEnoughHTTPRequestURLAttributesStable(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.request.method"] = http.MethodGet
	attributes["url.scheme"] = "http"
	attributes["client.address"] = "192.168.15.32"
	attributes["user_agent.original"] = "PostmanRuntime/7.21.0"
	attributes["url.path"] = "/users/junit"
	attributes["server.port"] = 443
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.URL)
	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Equal(t, http.MethodGet, *httpData.Request.Method)
	assert.Equal(t, "PostmanRuntime/7.21.0", *httpData.Request.UserAgent)
	contentLength := *httpData.Response.ContentLength.(*int64)
	assert.Equal(t, int64(12452), contentLength)
	assert.Equal(t, int64(200), *httpData.Response.Status)
	assert.NotNil(t, filtered)
}

func TestSpanWithNotEnoughHTTPRequestURLAttributesDuplicated(t *testing.T) {
	attributes := make(map[string]any)
	attributes["http.method"] = http.MethodGet
	attributes["http.request.method"] = http.MethodGet
	attributes["http.scheme"] = "http"
	attributes["url.scheme"] = "http"
	attributes["http.client_ip"] = "192.168.15.32"
	attributes["client.address"] = "192.168.15.32"
	attributes["http.user_agent"] = "PostmanRuntime/7.21.0"
	attributes["user_agent.original"] = "PostmanRuntime/7.21.0"
	attributes["http.target"] = "/users/junit"
	attributes["url.path"] = "/users/junit"
	attributes["net.host.port"] = 443
	attributes["server.port"] = 443
	attributes["net.peer.port"] = 8080
	attributes["http.response.status_code"] = 200
	attributes["http.response.status_code"] = 200
	span := constructHTTPServerSpan(attributes)
	timeEvents := constructTimedEventsWithReceivedMessageEvent(span.EndTimestamp())
	timeEvents.CopyTo(span.Events())

	filtered, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.URL)
	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Equal(t, http.MethodGet, *httpData.Request.Method)
	assert.Equal(t, "PostmanRuntime/7.21.0", *httpData.Request.UserAgent)
	contentLength := *httpData.Response.ContentLength.(*int64)
	assert.Equal(t, int64(12452), contentLength)
	assert.Equal(t, int64(200), *httpData.Response.Status)
	assert.NotNil(t, filtered)
}

func TestSpanWithClientAddrWithoutNetworkPeerAddr(t *testing.T) {
	attributes := make(map[string]any)
	attributes["url.full"] = "https://api.example.com/users/junit"
	attributes["client.address"] = "192.168.15.32"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Equal(t, aws.Bool(true), httpData.Request.XForwardedFor)
}

func TestSpanWithClientAddrAndNetworkPeerAddr(t *testing.T) {
	attributes := make(map[string]any)
	attributes["url.full"] = "https://api.example.com/users/junit"
	attributes["client.address"] = "192.168.15.32"
	attributes["network.peer.address"] = "192.168.15.32"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Equal(t, "192.168.15.32", *httpData.Request.ClientIP)
	assert.Nil(t, httpData.Request.XForwardedFor)
}

func TestSpanWithClientAddrNotIP(t *testing.T) {
	attributes := make(map[string]any)
	attributes["url.full"] = "https://api.example.com/users/junit"
	attributes["client.address"] = "api.example.com"
	attributes["network.peer.address"] = "api.example.com"
	span := constructHTTPServerSpan(attributes)

	_, httpData := makeHTTP(span)

	assert.Nil(t, httpData.Request.ClientIP)
	assert.Nil(t, httpData.Request.XForwardedFor)
}

func constructHTTPClientSpan(attributes map[string]any) ptrace.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
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

func constructHTTPServerSpan(attributes map[string]any) ptrace.Span {
	endTime := time.Now().Round(time.Second)
	startTime := endTime.Add(-90 * time.Second)
	spanAttributes := constructSpanAttributes(attributes)

	span := ptrace.NewSpan()
	span.SetTraceID(newTraceID())
	span.SetSpanID(newSegmentID())
	span.SetParentSpanID(newSegmentID())
	span.SetName("/users/junit")
	span.SetKind(ptrace.SpanKindServer)
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(endTime))

	status := ptrace.NewStatus()
	status.SetCode(0)
	status.SetMessage("OK")
	status.CopyTo(span.Status())

	spanAttributes.CopyTo(span.Attributes())
	return span
}

func TestMakeHTTPMixedConventionsUseDeterministicPrecedence(t *testing.T) {
	for _, mode := range []string{"default", "dual", "stable"} {
		t.Run(mode, func(t *testing.T) {
			if mode != "default" {
				enableGate(t, metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate)
			}
			if mode == "stable" {
				enableGate(t, metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate)
			}
			for _, reverse := range []bool{false, true} {
				for _, kind := range []ptrace.SpanKind{ptrace.SpanKindClient, ptrace.SpanKindServer} {
					span := constructHTTPClientSpan(nil)
					span.SetKind(kind)
					putOrderedAttributes(span.Attributes(), map[string]any{
						"http.method":               "POST",
						"http.request.method":       "GET",
						"http.status_code":          int64(429),
						"http.response.status_code": int64(503),
						"http.user_agent":           "legacy-agent",
						"user_agent.original":       "new-agent",
						"http.url":                  "http://old.example.com/old",
						"url.full":                  "https://new.example.com/new",
						"http.client_ip":            "192.0.2.1",
						"client.address":            "192.0.2.2",
					}, reverse)

					_, httpData := makeHTTP(span)
					require.NotNil(t, httpData)
					isError, isFault, isThrottle, _, _ := makeCause(span, nil, pcommon.NewResource())
					if mode == "default" {
						assert.Equal(t, "POST", *httpData.Request.Method)
						assert.Equal(t, int64(429), *httpData.Response.Status)
						assert.Equal(t, "legacy-agent", *httpData.Request.UserAgent)
						assert.Equal(t, "http://old.example.com/old", *httpData.Request.URL)
						assert.Equal(t, "192.0.2.1", *httpData.Request.ClientIP)
						assert.True(t, isError)
						assert.False(t, isFault)
						assert.True(t, isThrottle)
					} else {
						assert.Equal(t, "GET", *httpData.Request.Method)
						assert.Equal(t, int64(503), *httpData.Response.Status)
						assert.Equal(t, "new-agent", *httpData.Request.UserAgent)
						assert.Equal(t, "https://new.example.com/new", *httpData.Request.URL)
						assert.Equal(t, "192.0.2.2", *httpData.Request.ClientIP)
						assert.False(t, isError)
						assert.True(t, isFault)
						assert.False(t, isThrottle)
					}
				}
			}
		})
	}
}

func TestMakeHTTPServerURLPrefersStableAttributes(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate)
	for _, reverse := range []bool{false, true} {
		span := constructHTTPServerSpan(nil)
		putOrderedAttributes(span.Attributes(), map[string]any{
			"http.request.method": "GET",
			"http.scheme":         "http",
			"url.scheme":          "https",
			"http.host":           "old.example.com:8080",
			"net.host.name":       "old.example.com",
			"net.host.port":       int64(8080),
			"host.name":           "collector.example.com",
			"server.address":      "new.example.com",
			"server.port":         int64(8443),
			"http.target":         "/old?old=1",
			"url.path":            "/new",
			"url.query":           "new=1",
		}, reverse)
		_, httpData := makeHTTP(span)
		require.NotNil(t, httpData)
		assert.Equal(t, "https://new.example.com:8443/new?new=1", *httpData.Request.URL)
	}
}

func TestMakeHTTPStableResponseBodySize(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate)
	span := constructHTTPClientSpan(map[string]any{
		"http.request.method":     "GET",
		"http.response.body.size": 123,
	})
	_, httpData := makeHTTP(span)
	require.NotNil(t, httpData)
	assert.Equal(t, int64(123), *httpData.Response.ContentLength.(*int64))
}

func TestMakeHTTPIPv6URLUsesNetworkPeerOrServerAddress(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate)
	for _, kind := range []ptrace.SpanKind{ptrace.SpanKindClient, ptrace.SpanKindServer} {
		attributes := map[string]any{
			"http.request.method":  "GET",
			"url.scheme":           "https",
			"url.path":             "/orders",
			"network.peer.address": "2001:db8::1",
			"network.peer.port":    8443,
		}
		if kind == ptrace.SpanKindServer {
			attributes["server.address"] = "2001:db8::2"
			attributes["server.port"] = 8443
		}
		span := constructHTTPClientSpan(attributes)
		span.SetKind(kind)
		_, httpData := makeHTTP(span)
		require.NotNil(t, httpData)
		if kind == ptrace.SpanKindClient {
			assert.Equal(t, "https://[2001:db8::1]:8443/orders", *httpData.Request.URL)
		} else {
			assert.Equal(t, "https://[2001:db8::2]:8443/orders", *httpData.Request.URL)
		}
	}
}

func TestMakeHTTPStableClientEndpointWithDefaultGates(t *testing.T) {
	span := constructHTTPClientSpan(map[string]any{
		"http.request.method": "GET",
		"url.scheme":          "https",
		"url.path":            "/orders",
		"server.address":      "api.example.com",
		"server.port":         8443,
	})
	_, httpData := makeHTTP(span)
	require.NotNil(t, httpData)
	assert.Equal(t, "https://api.example.com:8443/orders", *httpData.Request.URL)
}

func TestMakeHTTPClientIPPrefersStableNetworkPeer(t *testing.T) {
	enableGate(t, metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate)
	for _, reverse := range []bool{false, true} {
		span := constructHTTPServerSpan(nil)
		putOrderedAttributes(span.Attributes(), map[string]any{
			"http.request.method":  "GET",
			"net.peer.ip":          "192.0.2.1",
			"network.peer.address": "192.0.2.2",
		}, reverse)
		_, httpData := makeHTTP(span)
		require.NotNil(t, httpData)
		assert.Equal(t, "192.0.2.2", *httpData.Request.ClientIP)
	}
}

func TestMakeHTTPNonHTTPAttributesPreserved(t *testing.T) {
	for _, attributes := range []map[string]any{
		{"db.system.name": "postgresql", "server.address": "db.example.com", "server.port": int64(5432)},
		{"rpc.system.name": "grpc", "server.address": "rpc.example.com", "server.port": int64(50051)},
	} {
		span := constructHTTPClientSpan(attributes)
		filtered, httpData := makeHTTP(span)
		require.Nil(t, httpData)
		for key, value := range span.Attributes().All() {
			require.Contains(t, filtered, key)
			assert.Equal(t, value.AsRaw(), filtered[key].AsRaw(), key)
		}
	}
}

// pdata preserves insertion order; exercise both orders without relying on
// randomized Go map iteration to expose precedence bugs.
func putOrderedAttributes(attributes pcommon.Map, values map[string]any, reverse bool) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if reverse {
		slices.Reverse(keys)
	}
	for _, key := range keys {
		switch value := values[key].(type) {
		case string:
			attributes.PutStr(key, value)
		case int64:
			attributes.PutInt(key, value)
		}
	}
}
