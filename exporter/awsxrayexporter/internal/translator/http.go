// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"maps"
	"net"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/metadata"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func makeHTTP(span ptrace.Span) (map[string]pcommon.Value, *awsxray.HTTPData) {
	var (
		info = awsxray.HTTPData{
			Request:  &awsxray.RequestData{},
			Response: &awsxray.ResponseData{},
		}
		filtered = make(map[string]pcommon.Value)
		urlParts = make(map[string]string)
	)

	if span.Attributes().Len() == 0 {
		return filtered, nil
	}

	hasHTTP := false
	hasHTTPRequestURLAttributes := false
	hasNetPeerAddr := false

	for key, value := range span.Attributes().All() {
		switch key {
		case string("http.method"), string(conventions.HTTPRequestMethodKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.method") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			selected, _ := httpAttribute(span.Attributes(), string(conventions.HTTPRequestMethodKey), "http.method")
			info.Request.Method = awsxray.String(selected.Str())
			hasHTTP = true
		case string("http.client_ip"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			info.Request.ClientIP = awsxray.String(value.Str())
			hasHTTP = true
		case string("http.user_agent"), string(conventions.UserAgentOriginalKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.user_agent") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			selected, _ := httpAttribute(span.Attributes(), string(conventions.UserAgentOriginalKey), "http.user_agent")
			info.Request.UserAgent = awsxray.String(selected.Str())
			hasHTTP = true
		case string("http.status_code"), string(conventions.HTTPResponseStatusCodeKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.status_code") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			selected, _ := httpAttribute(span.Attributes(), string(conventions.HTTPResponseStatusCodeKey), "http.status_code")
			info.Response.Status = aws.Int64(selected.Int())
			hasHTTP = true
		case string("http.url"), string(conventions.URLFullKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.url") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string("http.scheme"), string(conventions.URLSchemeKey):
			// TODO: Remove v0 key handling when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if key == string("http.scheme") && metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
		case string("http.host"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string("http.target"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
		case string("http.server_name"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
			hasHTTPRequestURLAttributes = true
		case string("net.host.port"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTP = true
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case string(conventions.HostNameKey):
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string("net.host.name"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string("net.peer.name"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string("net.peer.port"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			urlParts[key] = value.Str()
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		case string("net.peer.ip"):
			// TODO: Remove when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
			if metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
				filtered[key] = value
				break
			}
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if info.Request.ClientIP == nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			}
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
			hasNetPeerAddr = true
		case string(conventions.NetworkPeerAddressKey):
			// Prefer HTTP forwarded information (AttributeHTTPClientIP) when present.
			if net.ParseIP(value.Str()) != nil {
				if info.Request.ClientIP == nil {
					info.Request.ClientIP = awsxray.String(value.Str())
				}
				hasHTTPRequestURLAttributes = true
				hasNetPeerAddr = true
				urlParts[key] = value.Str()
			} else {
				filtered[key] = value
			}
		case string(conventions.NetworkPeerPortKey):
			if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
				urlParts[key] = value.Str()
				if urlParts[key] == "" {
					urlParts[key] = strconv.FormatInt(value.Int(), 10)
				}
			} else {
				filtered[key] = value
			}
		case string(conventions.ClientAddressKey):
			if net.ParseIP(value.Str()) != nil {
				info.Request.ClientIP = awsxray.String(value.Str())
			} else {
				filtered[key] = value
			}
		case string(conventions.URLPathKey):
			urlParts[key] = value.Str()
			hasHTTP = true
		case string(conventions.URLQueryKey):
			urlParts[key] = value.Str()
			hasHTTP = true
		case string(conventions.ServerAddressKey):
			urlParts[key] = value.Str()
			hasHTTPRequestURLAttributes = true
		case string(conventions.ServerPortKey):
			urlParts[key] = value.Str()
			if urlParts[key] == "" {
				urlParts[key] = strconv.FormatInt(value.Int(), 10)
			}
		default:
			filtered[key] = value
		}
	}

	// Resolve forwarded client information after iteration so an earlier
	// network peer or legacy client IP cannot override the preferred value.
	if value, ok := httpAttribute(span.Attributes(), string(conventions.ClientAddressKey), "http.client_ip"); ok && net.ParseIP(value.Str()) != nil {
		info.Request.ClientIP = awsxray.String(value.Str())
	} else if value, ok := httpAttribute(span.Attributes(), string(conventions.NetworkPeerAddressKey), "net.peer.ip"); ok && net.ParseIP(value.Str()) != nil {
		info.Request.ClientIP = awsxray.String(value.Str())
	}

	if !hasNetPeerAddr && info.Request.ClientIP != nil {
		info.Request.XForwardedFor = aws.Bool(true)
	}

	if !hasHTTP {
		// Didn't have any HTTP-specific information so don't need to fill it in segment
		// Shared network attributes still belong to DB and RPC spans.
		maps.Insert(filtered, span.Attributes().All())
		return filtered, nil
	}

	if hasHTTPRequestURLAttributes {
		if span.Kind() == ptrace.SpanKindServer {
			info.Request.URL = awsxray.String(constructServerURL(urlParts))
		} else {
			info.Request.URL = awsxray.String(constructClientURL(urlParts))
		}
	}

	info.Response.ContentLength = aws.Int64(extractResponseSizeFromEvents(span))

	return filtered, &info
}

func extractResponseSizeFromEvents(span ptrace.Span) int64 {
	// Support instrumentation that sets response size in span or as an event.
	size := extractResponseSizeFromAttributes(span.Attributes())
	if size != 0 {
		return size
	}
	for i := 0; i < span.Events().Len(); i++ {
		event := span.Events().At(i)
		size = extractResponseSizeFromAttributes(event.Attributes())
		if size != 0 {
			return size
		}
	}
	return size
}

func extractResponseSizeFromAttributes(attributes pcommon.Map) int64 {
	if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if size, ok := attributes.Get(string(conventions.HTTPResponseBodySizeKey)); ok {
			return size.Int()
		}
	}
	if !metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if size, ok := attributes.Get("http.response_content_length"); ok {
			return size.Int()
		}
	}
	typeVal, ok := attributes.Get("message.type")
	if ok && typeVal.Str() == "RECEIVED" {
		if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
			if sizeVal, ok := attributes.Get(string(conventions.MessagingMessageBodySizeKey)); ok {
				return sizeVal.Int()
			}
		}
		// TODO: Remove old key when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
		if !metadata.ExporterAwsxrayDontEmitV0HTTPNetworkConventionsFeatureGate.IsEnabled() {
			if sizeVal, ok := attributes.Get(string("messaging.message_payload_size_bytes")); ok {
				return sizeVal.Int()
			}
		}
	}
	return 0
}

func constructClientURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for client spans described in
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md#http-client

	url, ok := httpURLPart(urlParts, string(conventions.URLFullKey), "http.url")
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := httpURLPart(urlParts, string(conventions.URLSchemeKey), "http.scheme")
	if !ok {
		scheme = "http"
	}
	port := ""
	// server.address/server.port replace net.peer.name/net.peer.port for client spans:
	// https://opentelemetry.io/docs/specs/semconv/non-normative/http-migration/#http-client-span-attributes
	emitV1 := metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled()
	v1Host, hasV1Host := urlParts[string(conventions.ServerAddressKey)]
	// TODO: Remove old-key lookups when exporter.awsxray.DontEmitV0HTTPNetworkConventions is removed.
	host, ok := urlParts[string("http.host")]
	if !ok || (emitV1 && hasV1Host) {
		if emitV1 && hasV1Host {
			host = v1Host
		} else {
			host, ok = urlParts[string("net.peer.name")]
			if !ok {
				host, ok = urlParts[string("net.peer.ip")]
				if !ok {
					host = v1Host
					if host == "" {
						host = urlParts[string(conventions.NetworkPeerAddressKey)]
					}
				}
			}
		}
		port, ok = httpURLPart(urlParts, string(conventions.ServerPortKey), "net.peer.port")
		if !ok {
			port = urlParts[string(conventions.NetworkPeerPortKey)]
		}
	}
	url = scheme + "://" + httpURLAuthority(host, port, scheme)
	return appendHTTPTarget(url, urlParts)
}

func constructServerURL(urlParts map[string]string) string {
	// follows OpenTelemetry specification-defined combinations for server spans described in
	// https://github.com/open-telemetry/semantic-conventions/blob/main/docs/http/http-spans.md#http-server

	url, ok := httpURLPart(urlParts, string(conventions.URLFullKey), "http.url")
	if ok {
		// full URL available so no need to assemble
		return url
	}

	scheme, ok := httpURLPart(urlParts, string(conventions.URLSchemeKey), "http.scheme")
	if !ok {
		scheme = "http"
	}
	port := ""
	if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if host, hasServerAddress := urlParts[string(conventions.ServerAddressKey)]; hasServerAddress {
			port, _ = httpURLPart(urlParts, string(conventions.ServerPortKey), "net.host.port")
			url = scheme + "://" + httpURLAuthority(host, port, scheme)
			return appendHTTPTarget(url, urlParts)
		}
	}
	host, ok := urlParts[string("http.host")]
	if !ok {
		host, ok = urlParts[string("http.server_name")]
		if !ok {
			host, ok = urlParts[string("net.host.name")]
			if !ok {
				host, ok = urlParts[string(conventions.HostNameKey)]
				if !ok {
					host = urlParts[string(conventions.ServerAddressKey)]
				}
			}
		}
		port, ok = urlParts[string("net.host.port")]
		if !ok {
			port, ok = urlParts[string(conventions.ServerPortKey)]
			if !ok {
				port = ""
			}
		}
	}
	url = scheme + "://" + httpURLAuthority(host, port, scheme)
	return appendHTTPTarget(url, urlParts)
}

func httpURLPart(parts map[string]string, stableKey, legacyKey string) (string, bool) {
	if metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if value, ok := parts[stableKey]; ok {
			return value, true
		}
	}
	if value, ok := parts[legacyKey]; ok {
		return value, true
	}
	value, ok := parts[stableKey]
	return value, ok
}

func httpURLAuthority(host, port, scheme string) string {
	if !metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() {
		if port != "" && (scheme != "http" || port != "80") && (scheme != "https" || port != "443") {
			return host + ":" + port
		}
		return host
	}
	if port != "" && (scheme != "http" || port != "80") && (scheme != "https" || port != "443") {
		return net.JoinHostPort(strings.Trim(host, "[]"), port)
	}
	if net.ParseIP(host) != nil && strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func appendHTTPTarget(url string, parts map[string]string) string {
	path, hasPath := parts[string(conventions.URLPathKey)]
	query, hasQuery := parts[string(conventions.URLQueryKey)]
	if target, ok := parts["http.target"]; ok &&
		(!metadata.ExporterAwsxrayEmitV1HTTPNetworkConventionsFeatureGate.IsEnabled() || (!hasPath && !hasQuery)) {
		return url + target
	}
	if hasPath {
		url += path
	} else {
		url += "/"
	}
	if hasQuery && query != "" {
		if !strings.HasPrefix(query, "?") {
			query = "?" + query
		}
		url += query
	}
	return url
}
