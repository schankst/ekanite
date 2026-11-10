package input

import (
	"bytes"
	"reflect"
	"testing"
)

func Test_Formats(t *testing.T) {
	var p *Parser
	mismatched := func(rtrnd string, intnd string, intndA string) {
		if intndA != "" {
			t.Fatalf("Parser format %v does not match the intended format %v.\n", rtrnd, intnd)
		}
		t.Fatalf("Parser format %v does not match the intended format %v (same as: %v).\n", rtrnd, intndA, intnd)
	}

	for _, f := range supportedFormats() {
		p, _ = NewParser(f[0])
		if p.Fmt != f[1] {
			mismatched(p.Fmt, f[0], f[1])
		}
	}
	for _, f := range supportedFormats() {
		p, _ = NewParser(f[1])
		if p.Fmt != f[1] {
			mismatched(p.Fmt, f[1], "")
		}
	}
	_, err := NewParser("unknown-format")
	if err == nil {
		t.Fatalf("parser successfully created with invalid format")
	}
}

func Test_Parsing(t *testing.T) {
	tests := []struct {
		fmt      string
		message  string
		expected map[string]interface{}
		fail     bool
	}{
		{
			fmt:     "syslog",
			message: `<134>1 2003-08-24T05:14:15.000003-07:00 ubuntu sshd 1999 - password accepted`,
			expected: map[string]interface{}{
				"priority":   134,
				"facility":   16,
				"severity":   6,
				"version":    1,
				"timestamp":  "2003-08-24T05:14:15.000003-07:00",
				"host":       "ubuntu",
				"app":        "sshd",
				"pid":        1999,
				"message_id": nil,
				"message":    "password accepted",
			},
		},
		{
			fmt:     "syslog",
			message: `<33>5 1985-04-12T23:20:50.52Z test.com cron 304 - password accepted`,
			expected: map[string]interface{}{
				"priority":   33,
				"facility":   4,
				"severity":   1,
				"version":    5,
				"timestamp":  "1985-04-12T23:20:50.52Z",
				"host":       "test.com",
				"app":        "cron",
				"pid":        304,
				"message_id": nil,
				"message":    "password accepted",
			},
		},
		{
			fmt:     "syslog",
			message: `<1>0 1985-04-12T19:20:50.52-04:00 test.com cron 65535 - password accepted`,
			expected: map[string]interface{}{
				"priority":   1,
				"facility":   0,
				"severity":   1,
				"version":    0,
				"timestamp":  "1985-04-12T19:20:50.52-04:00",
				"host":       "test.com",
				"app":        "cron",
				"pid":        65535,
				"message_id": nil,
				"message":    "password accepted",
			},
		},
		{
			fmt:     "syslog",
			message: `<1>0 2003-10-11T22:14:15.003Z test.com cron 65535 msgid1234 password accepted`,
			expected: map[string]interface{}{
				"priority":   1,
				"facility":   0,
				"severity":   1,
				"version":    0,
				"timestamp":  "2003-10-11T22:14:15.003Z",
				"host":       "test.com",
				"app":        "cron",
				"pid":        65535,
				"message_id": "msgid1234",
				"message":    "password accepted",
			},
		},
		{
			fmt:     "syslog",
			message: `<1>0 2003-08-24T05:14:15.000003-07:00 test.com cron 65535 - JVM NPE\nsome_file.java:48\n\tsome_other_file.java:902`,
			expected: map[string]interface{}{
				"priority":   1,
				"facility":   0,
				"severity":   1,
				"version":    0,
				"timestamp":  "2003-08-24T05:14:15.000003-07:00",
				"host":       "test.com",
				"app":        "cron",
				"pid":        65535,
				"message_id": nil,
				"message":    `JVM NPE\nsome_file.java:48\n\tsome_other_file.java:902`,
			},
		},
		{
			fmt:     "syslog",
			message: `<27>1 2015-03-02T22:53:45-08:00 localhost.localdomain puppet-agent 5334 - mirrorurls.extend(list(self.metalink_data.urls()))`,
			expected: map[string]interface{}{
				"priority":   27,
				"facility":   3,
				"severity":   3,
				"version":    1,
				"timestamp":  "2015-03-02T22:53:45-08:00",
				"host":       "localhost.localdomain",
				"app":        "puppet-agent",
				"pid":        5334,
				"message_id": nil,
				"message":    "mirrorurls.extend(list(self.metalink_data.urls()))",
			},
		},
		{
			fmt:     "syslog",
			message: `<29>1 2015-03-03T06:49:08-08:00 localhost.localdomain puppet-agent 51564 - (/Stage[main]/Users_prd/Ssh_authorized_key[1063-username]) Dependency Group[group] has failures: true`,
			expected: map[string]interface{}{
				"priority":   29,
				"facility":   3,
				"severity":   5,
				"version":    1,
				"timestamp":  "2015-03-03T06:49:08-08:00",
				"host":       "localhost.localdomain",
				"app":        "puppet-agent",
				"pid":        51564,
				"message_id": nil,
				"message":    "(/Stage[main]/Users_prd/Ssh_authorized_key[1063-username]) Dependency Group[group] has failures: true",
			},
		},
		{
			fmt:     "syslog",
			message: `<142>1 2015-03-02T22:23:07-08:00 localhost.localdomain Keepalived_vrrp 21125 - VRRP_Instance(VI_1) ignoring received advertisement...`,
			expected: map[string]interface{}{
				"priority":   142,
				"facility":   17,
				"severity":   6,
				"version":    1,
				"timestamp":  "2015-03-02T22:23:07-08:00",
				"host":       "localhost.localdomain",
				"app":        "Keepalived_vrrp",
				"pid":        21125,
				"message_id": nil,
				"message":    "VRRP_Instance(VI_1) ignoring received advertisement...",
			},
		},
		{
			fmt:     "syslog",
			message: `<142>1 2015-03-02T22:23:07-08:00 localhost.localdomain Keepalived_vrrp 21125 - HEAD /wp-login.php HTTP/1.1" 200 167 "http://www.philipotoole.com/" "Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.11 (KHTML, like Gecko) Chrome/23.0.1271.97 Safari/537.11`,
			expected: map[string]interface{}{
				"priority":   142,
				"facility":   17,
				"severity":   6,
				"version":    1,
				"timestamp":  "2015-03-02T22:23:07-08:00",
				"host":       "localhost.localdomain",
				"app":        "Keepalived_vrrp",
				"pid":        21125,
				"message_id": nil,
				"message":    `HEAD /wp-login.php HTTP/1.1" 200 167 "http://www.philipotoole.com/" "Mozilla/5.0 (Windows NT 6.1) AppleWebKit/537.11 (KHTML, like Gecko) Chrome/23.0.1271.97 Safari/537.11`,
			},
		},
		{
			fmt:     "syslog",
			message: `<134>0 2015-05-05T21:20:00.493320+00:00 fisher apache-access - - 173.247.206.174 - - [05/May/2015:21:19:52 +0000] "GET /2013/11/ HTTP/1.1" 200 22056 "http://www.philipotoole.com/" "Wget/1.15 (linux-gnu)"`,
			expected: map[string]interface{}{
				"priority":   134,
				"facility":   16,
				"severity":   6,
				"version":    0,
				"timestamp":  "2015-05-05T21:20:00.493320+00:00",
				"host":       "fisher",
				"app":        "apache-access",
				"pid":        nil,
				"message_id": nil,
				"message":    `173.247.206.174 - - [05/May/2015:21:19:52 +0000] "GET /2013/11/ HTTP/1.1" 200 22056 "http://www.philipotoole.com/" "Wget/1.15 (linux-gnu)"`,
			},
		},
		{
			fmt:     "syslog",
			message: `<134>0 2017-06-04T14:09:13+02:00 192.168.1.217 filterlog - - 67,,,0,vtnet0,match,pass,out,4,0x0,,127,3328,0,DF,6,tcp,366,192.168.1.66,31.13.86.4,50800,443,326,PA,1912507082:1912507408,2077294259,257,,`,
			expected: map[string]interface{}{
				"priority":   134,
				"facility":   16,
				"severity":   6,
				"version":    0,
				"timestamp":  "2017-06-04T14:09:13+02:00",
				"host":       "192.168.1.217",
				"app":        "filterlog",
				"pid":        nil,
				"message_id": nil,
				"message":    `67,,,0,vtnet0,match,pass,out,4,0x0,,127,3328,0,DF,6,tcp,366,192.168.1.66,31.13.86.4,50800,443,326,PA,1912507082:1912507408,2077294259,257,,`,
			},
		},
		{
			fmt:     "syslog",
			message: `<33>7 2013-09-04T10:25:52.618085 test.com cron web-01 - password accepted`,
			expected: map[string]interface{}{
				"priority":   33,
				"facility":   4,
				"severity":   1,
				"version":    7,
				"timestamp":  "2013-09-04T10:25:52.618085",
				"host":       "test.com",
				"app":        "cron",
				"pid":        "web-01",
				"message_id": nil,
				"message":    "password accepted",
			},
		},
		{
			fmt:     "syslog",
			message: `<134> 2013-09-04T10:25:52.618085 ubuntu sshd 1999 - password accepted`,
			fail:    true,
		},
		{
			fmt:     "syslog",
			message: `<33> 7 2013-09-04T10:25:52.618085 test.com cron 304 - password accepted`,
			fail:    true,
		},
		{
			fmt:     "syslog",
			message: `<33> 7 2013-09-04T10:25:52.618085 test.com cron 304 $ password accepted`,
			fail:    true,
		},
		{
			fmt:     "syslog",
			message: `<33> 7 2013-09-04T10:25:52.618085 test.com cron 304 - - password accepted`,
			fail:    true,
		},
		{
			fmt:     "syslog",
			message: `5:52.618085 test.com cron 65535 - password accepted`,
			fail:    true,
		},
	}

	for i, tt := range tests {
		p, _ := NewParser(tt.fmt)
		t.Logf("using %d\n", i+1)
		ok := p.Parse(bytes.NewBufferString(tt.message).Bytes())
		if tt.fail {
			if ok {
				t.Error("\n\nParser should fail.\n")
			}
		} else {
			if !ok {
				t.Error("\n\nParser should succeed.\n")
			}
		}
		if !tt.fail && !reflect.DeepEqual(tt.expected, p.Result) {
			t.Logf("%v", p.Result)
			t.Logf("%v", tt.expected)
			t.Error("\n\nParser result does not match expected result.\n")
		}
	}
}
