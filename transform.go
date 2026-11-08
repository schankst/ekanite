package ekanite

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"sync"
)

// TransformConfig controls data minimization and pseudonymization before an
// event is indexed (DSGVO Issues #93/#95).
//
// RawMode selects how the original log line is kept:
//   - "keep" (default): the raw line is indexed as full-text Message and
//     returned as the search result source.
//   - "drop": no raw line is indexed; the search result source is a
//     serialized view of the parsed (and possibly anonymized) fields only.
//   - "strip" is treated as "keep": the raw line is inseparable from the
//     full-text Message in this data model; removing the duplicate field
//     storage is achieved by "drop".
//
// Anonymize maps a field name to a transformation:
//   - "inet": mask the last octet/group of an IPv4/IPv6 address (10.1.2.3 ->
//     10.1.2.0). Only meaningful for IP-typed fields.
//   - "hmac": HMAC-SHA256 of the value, hex-encoded, keyed by AnonKey.
//
// When any field is anonymized, the raw line is not indexed or returned at all
// (it would leak the original values); only the transformed fields are stored.
type TransformConfig struct {
	RawMode   string
	Anonymize map[string]string // field name -> "inet" | "hmac"
	AnonKey   []byte
}

var (
	transformMu     sync.RWMutex
	transformConfig TransformConfig
)

// SetTransform installs the process-wide ingest transformation policy. It is
// set once from the CLI flags before the engine is opened.
func SetTransform(cfg TransformConfig) {
	transformMu.Lock()
	defer transformMu.Unlock()
	transformConfig = cfg
}

// transform returns the current policy.
func transform() TransformConfig {
	transformMu.RLock()
	defer transformMu.RUnlock()
	return transformConfig
}

// dropRaw reports whether the original raw line must be excluded from the
// document and from search-result sources (DSGVO #93/#95).
func (c TransformConfig) dropRaw() bool {
	return c.RawMode == "drop" || len(c.Anonymize) > 0
}

// anonymize applies the configured transformation to a single field value.
// Unconfigured fields are returned unchanged.
func (c TransformConfig) anonymize(field string, v any) any {
	if c.Anonymize == nil {
		return v
	}
	op, ok := c.Anonymize[field]
	if !ok {
		return v
	}
	s, ok := v.(string)
	if !ok {
		return v
	}
	switch op {
	case "inet":
		return maskIP(s)
	case "hmac":
		return hmacHex(s, c.AnonKey)
	}
	return v
}

// maskIP masks the last IPv4 octet (or IPv6 hextet) to zero. Non-IP input is
// returned unchanged so a wrongly configured field is not silently altered.
func maskIP(s string) string {
	ip := net.ParseIP(s)
	if ip == nil {
		return s
	}
	if v4 := ip.To4(); v4 != nil {
		v4[3] = 0
		return v4.String()
	}
	if v6 := ip.To16(); v6 != nil {
		v6[15] = 0
		return v6.String()
	}
	return s
}

// hmacHex returns the hex-encoded HMAC-SHA256 of v keyed by key. A missing or
// empty key produces an empty string (safer than an unkeyed hash: the value is
// simply not pseudonymized rather than weakly hashed).
func hmacHex(v string, key []byte) string {
	if len(key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(v))
	return hex.EncodeToString(mac.Sum(nil))
}

// serializedFields renders the parsed fields as compact JSON, used as the
// search-result source when the raw line is dropped (DSGVO #95). The fallback
// "raw" key (which carries the original unparsed line) is always excluded so
// raw data never leaks through a minimized source view.
func serializedFields(parsed map[string]any) []byte {
	out := make(map[string]any, len(parsed))
	for k, v := range parsed {
		if k == "raw" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil
	}
	return b
}
