package input

import (
	"testing"
)

/*
 * SyslogDelimiter tests.
 */

func Test_SyslogDelimiter(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "simple",
			line:     "<11>1 sshd is down\n<22>1 sshd is up\n<67>2 password accepted",
			expected: []string{"<11>1 sshd is down", "<22>1 sshd is up"},
		},
		{
			name:     "leading",
			line:     "password accepted for user root<12>1 sshd is down\n<145>1 sshd is up\n<67>2 password accepted",
			expected: []string{"<12>1 sshd is down", "<145>1 sshd is up"},
		},
		{
			name:     "CRLF",
			line:     "<12>1 sshd is down\r\n<145>1 sshd is up\r\n<67>2 password accepted",
			expected: []string{"<12>1 sshd is down", "<145>1 sshd is up"},
		},
		{
			name:     "stacktrace",
			line:     "<12>1 sshd is down\n<145>1 OOM on line 42, dummy.java\n\tclass_loader.jar\n<67>2 password accepted",
			expected: []string{"<12>1 sshd is down", "<145>1 OOM on line 42, dummy.java\n\tclass_loader.jar"},
		},
		{
			name:     "embedded",
			line:     "<12>1 sshd is <down>\n<145>1 sshd is up<33>4\n<67>2 password accepted",
			expected: []string{"<12>1 sshd is <down>", "<145>1 sshd is up<33>4"},
		},
		{
			// #18: A 4-digit PRI (<9999>) must still be recognized as a syslog
			// line start so misconfigured senders are not silently dropped.
			name:     "four_digit_pri",
			line:     "<9999>1 first line\n<22>1 second line\n",
			expected: []string{"<9999>1 first line"},
		},
	}

	for _, tt := range tests {
		d := NewSyslogDelimiter(256)
		events := []string{}

		for _, b := range tt.line {
			event, match := d.Push(byte(b))
			if match {
				events = append(events, event)
			}
		}

		if len(events) != len(tt.expected) {
			t.Errorf("test %s: failed to delimit '%s' as expected", tt.name, tt.line)
		} else {
			for i := 0; i < len(events); i++ {
				if events[i] != tt.expected[i] {
					t.Errorf("test %s: failed to delimit '%s', got %s, expected %s", tt.name, tt.line, events[i], tt.expected)
				}
			}
		}
	}
}

func TestSyslogDelimiter_flushRemainder(t *testing.T) {
	tests := []struct {
		name          string
		line          string
		expectedEvent string
		expectedMatch bool
	}{
		{
			name:          "flushRemainder zero",
			line:          "",
			expectedEvent: "",
			expectedMatch: false,
		},
		{
			name:          "flushRemainder no match",
			line:          "12\n",
			expectedEvent: "",
			expectedMatch: false,
		},
		{
			name:          "flushRemainder match",
			line:          "<12>3 ",
			expectedEvent: "<12>3 ",
			expectedMatch: true,
		},
		{
			name:          "flushRemainder rich match",
			line:          "<145>1 OOM on line 42, dummy.java\n\tclass_loader.jar",
			expectedEvent: "<145>1 OOM on line 42, dummy.java\n\tclass_loader.jar",
			expectedMatch: true,
		},
	}

	for _, tt := range tests {
		d := NewSyslogDelimiter(256)
		for _, c := range tt.line {
			d.Push(byte(c))
		}
		e, m := d.flushRemainder()
		if e != tt.expectedEvent || m != tt.expectedMatch {
			t.Errorf("test %s: flushRemainder test failed, got %s %v, expected %s %v", tt.name, e, m, tt.expectedEvent, tt.expectedMatch)
		}
	}
}
