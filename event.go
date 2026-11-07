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
func (e Event) Data() interface{} {
	doc := make(map[string]interface{}, len(e.Parsed)+4)
	for k, v := range e.Parsed {
		doc[k] = v
	}
	// Distinct structured fields (overwrite any parsed-field collisions with
	// the canonical document fields).
	doc["Message"] = e.Text
	doc["ReferenceTime"] = e.ReferenceTime()
	doc["ReceptionTime"] = e.ReceptionTime
	if e.SourceIP != "" {
		doc["source_ip"] = e.SourceIP
	}
	return doc
}

// Source returns the original received data.
func (e Event) Source() []byte {
	return []byte(e.Text)
}
