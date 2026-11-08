package ekanite

import (
	"fmt"

	"github.com/ekanite/ekanite/input"
)

// Event is a log message that can be indexed.
type Event struct {
	*input.Event
}

// NewEvent returns a new Event.
func NewEvent() *Event {
	return &Event{}
}

// ID returns a unique ID for the event.
func (e Event) ID() DocID {
	return DocID(fmt.Sprintf("%016x%016x",
		uint64(e.ReferenceTime().UnixNano()), uint64(e.Sequence)))
}

// Data returns the indexable data. In addition to the raw Message, all parsed
// fields (host, app, pid, msgid, facility, severity, source_ip, etc.), the
// source IP and the time fields are carried in the document so they are
// field-searchable (Issue #34). The Message keeps the raw line for full-text
// search; the structured fields enable `field:value` queries.
//
// With data minimization enabled (DSGVO #93/#95) the raw line is excluded and
// configured fields are anonymized before indexing, so original values do not
// appear in the index.
func (e Event) Data() any {
	cfg := transform()
	doc := make(map[string]any, len(e.Parsed)+4)
	for k, v := range e.Parsed {
		doc[k] = cfg.anonymize(k, v)
	}
	if !cfg.dropRaw() {
		doc["Message"] = e.Text
	}
	doc["ReferenceTime"] = e.ReferenceTime()
	doc["ReceptionTime"] = e.ReceptionTime
	if e.SourceIP != "" {
		doc["source_ip"] = cfg.anonymize("source_ip", e.SourceIP)
	}
	return doc
}

// Source returns the data stored for a search hit. By default this is the
// original raw line; with data minimization it is a serialized view of the
// (anonymized) parsed fields so no raw values leak into search results.
func (e Event) Source() []byte {
	cfg := transform()
	if cfg.dropRaw() {
		anonymized := make(map[string]any, len(e.Parsed)+1)
		for k, v := range e.Parsed {
			anonymized[k] = cfg.anonymize(k, v)
		}
		if e.SourceIP != "" {
			anonymized["source_ip"] = cfg.anonymize("source_ip", e.SourceIP)
		}
		return serializedFields(anonymized)
	}
	return []byte(e.Text)
}
