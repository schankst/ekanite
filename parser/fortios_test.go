package parser

import (
	"testing"
)

func TestFortiOSParse(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantOK    bool
		timestamp string
		host      string
		app       string
		sourceIP  string
		dstip     string
		logType   string
		subtype   string
		action    string
		level     string
		message   string
	}{
		{
			name:      "traffic quoted values",
			line:      `date=2023-01-01 time=12:00:01 devname="FGT80F" devid="FGT801234" logid="0100032001" type="traffic" subtype="forward" level="notice" vd="root" srcip=192.168.1.10 srcport=56789 srcintf="wan1" dstip=203.0.113.5 dstport=443 dstintf="port3" action="accept" policyid=17 sessionid=123456789 srcname="" dstname="" app="HTTPS"`,
			wantOK:    true,
			timestamp: "2023-01-01T12:00:01Z",
			host:      "FGT80F",
			app:       "HTTPS",
			sourceIP:  "192.168.1.10",
			dstip:     "203.0.113.5",
			logType:   "traffic",
			subtype:   "forward",
			action:    "accept",
			level:     "notice",
		},
		{
			name:      "traffic unquoted no timezone",
			line:      `date=2024-06-15 time=08:30:45 devname=FGT60E devid=FG60E4Q logid=0000000013 type=traffic subtype=local level=notice srcip=10.0.0.5 dstip=10.0.0.1 action=accept app=DNS`,
			wantOK:    true,
			timestamp: "2024-06-15T08:30:45Z",
			host:      "FGT60E",
			app:       "DNS",
			sourceIP:  "10.0.0.5",
			dstip:     "10.0.0.1",
			logType:   "traffic",
			subtype:   "local",
			action:    "accept",
			level:     "notice",
		},
		{
			name:      "event log",
			line:      `date=2023-03-10 time=14:22:33 devname="FGT100D" logid="0100044546" type="event" subtype="system" level="information" msg="Administrator admin logged in successfully from 192.168.1.20" action="login"`,
			wantOK:    true,
			timestamp: "2023-03-10T14:22:33Z",
			host:      "FGT100D",
			logType:   "event",
			subtype:   "system",
			action:    "login",
			level:     "information",
			message:   "Administrator admin logged in successfully from 192.168.1.20",
		},
		{
			name:      "utm waf webfilter",
			line:      `date=2023-07-04 time=09:11:22 devname="FGT-Edge" logid="0316013056" type="utm" subtype="webfilter" level="warning" srcip=172.16.0.50 dstip=93.184.216.34 action="blocked" app="HTTP.BROWSER" msg="URL belongs to a denied category in policy"`,
			wantOK:    true,
			timestamp: "2023-07-04T09:11:22Z",
			host:      "FGT-Edge",
			app:       "HTTP.BROWSER",
			sourceIP:  "172.16.0.50",
			dstip:     "93.184.216.34",
			logType:   "utm",
			subtype:   "webfilter",
			action:    "blocked",
			level:     "warning",
			message:   "URL belongs to a denied category in policy",
		},
		{
			name:      "threat ips",
			line:      `date=2023-08-20 time=23:59:59 devname="FGT-IPS" logid="0419016384" type="utm" subtype="ips" level="alert" srcip=198.51.100.7 dstip=10.10.10.10 action="dropped" app="HTTP" attack="Apache.Log4j.Remote.Code.Execution" msg="Anomaly detected"`,
			wantOK:    true,
			timestamp: "2023-08-20T23:59:59Z",
			host:      "FGT-IPS",
			app:       "HTTP",
			sourceIP:  "198.51.100.7",
			dstip:     "10.10.10.10",
			logType:   "utm",
			subtype:   "ips",
			action:    "dropped",
			level:     "alert",
			message:   "Anomaly detected",
		},
		{
			name:      "time with timezone offset",
			line:      `date=2023-11-01 time=16:45:00+02:00 devname="FGT-TZ" type="traffic" subtype="forward" level="notice" srcip=10.1.1.1 dstip=10.2.2.2 action="accept" app="SSH"`,
			wantOK:    true,
			timestamp: "2023-11-01T16:45:00+02:00",
			host:      "FGT-TZ",
			app:       "SSH",
			sourceIP:  "10.1.1.1",
			dstip:     "10.2.2.2",
			logType:   "traffic",
			subtype:   "forward",
			action:    "accept",
			level:     "notice",
		},
		{
			name:      "time with negative timezone offset",
			line:      `date=2023-11-01 time=07:45:00-07:00 devname=FGT-WEST type=event subtype=system level=notice action=test`,
			wantOK:    true,
			timestamp: "2023-11-01T07:45:00-07:00",
			host:      "FGT-WEST",
			logType:   "event",
			subtype:   "system",
			action:    "test",
			level:     "notice",
		},
		{
			name:      "time with Z timezone",
			line:      `date=2023-05-05 time=00:00:00Z devname="FGT-Z" type="traffic" subtype="forward" level="notice" srcip=1.2.3.4 dstip=5.6.7.8 action="deny"`,
			wantOK:    true,
			timestamp: "2023-05-05T00:00:00Z",
			host:      "FGT-Z",
			sourceIP:  "1.2.3.4",
			dstip:     "5.6.7.8",
			logType:   "traffic",
			subtype:   "forward",
			action:    "deny",
			level:     "notice",
		},
		{
			name:      "mixed quoted and unquoted with msg fallback",
			line:      `date=2024-01-15 time=10:10:10 devname=BranchFW devid="FGT123" type=traffic subtype=forward level=notice srcip="192.168.50.1" dstip="8.8.8.8" action=accept app=ping`,
			wantOK:    true,
			timestamp: "2024-01-15T10:10:10Z",
			host:      "BranchFW",
			app:       "ping",
			sourceIP:  "192.168.50.1",
			dstip:     "8.8.8.8",
			logType:   "traffic",
			subtype:   "forward",
			action:    "accept",
			level:     "notice",
		},
		{
			name:      "utm virus threat",
			line:      `date=2024-02-28 time=18:05:00 devname="FGT-AV" logid="0211008192" type="utm" subtype="virus" level="warning" srcip=203.0.113.99 dstip=192.168.2.20 action="blocked" app="SMTP" msg="File reported infected by Antivirus scan"`,
			wantOK:    true,
			timestamp: "2024-02-28T18:05:00Z",
			host:      "FGT-AV",
			app:       "SMTP",
			sourceIP:  "203.0.113.99",
			dstip:     "192.168.2.20",
			logType:   "utm",
			subtype:   "virus",
			action:    "blocked",
			level:     "warning",
			message:   "File reported infected by Antivirus scan",
		},
		{
			name:      "unknown keys preserved",
			line:      `date=2024-03-01 time=01:02:03 devname="FGT-X" type=event subtype=system level=notice customkey=customval action=info`,
			wantOK:    true,
			timestamp: "2024-03-01T01:02:03Z",
			host:      "FGT-X",
			logType:   "event",
			subtype:   "system",
			action:    "info",
			level:     "notice",
		},
		{name: "empty line", line: "", wantOK: false},
		{name: "random text", line: "this is not a fortios log line", wantOK: false},
		{name: "missing time", line: `date=2023-01-01 devname="FGT" type=traffic`, wantOK: false},
		{name: "missing date", line: `time=12:00:01 devname="FGT" type=traffic`, wantOK: false},
		{name: "malformed date time", line: `date=not-a-date time=not-a-time devname="FGT"`, wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &FortiOS{}
			p.Init()
			result := map[string]interface{}{}
			p.Parse([]byte(tt.line), &result)
			if !tt.wantOK {
				if len(result) != 0 {
					t.Fatalf("expected unparsable, got result: %v", result)
				}
				return
			}
			if len(result) == 0 {
				t.Fatalf("expected parsed result, got empty")
			}
			if tt.timestamp != "" && result["timestamp"] != tt.timestamp {
				t.Fatalf("timestamp: got %v, want %v", result["timestamp"], tt.timestamp)
			}
			if tt.host != "" && result["host"] != tt.host {
				t.Fatalf("host: got %v, want %v", result["host"], tt.host)
			}
			if tt.app != "" && result["app"] != tt.app {
				t.Fatalf("app: got %v, want %v", result["app"], tt.app)
			}
			if tt.sourceIP != "" && result["source_ip"] != tt.sourceIP {
				t.Fatalf("source_ip: got %v, want %v", result["source_ip"], tt.sourceIP)
			}
			if tt.dstip != "" && result["dstip"] != tt.dstip {
				t.Fatalf("dstip: got %v, want %v", result["dstip"], tt.dstip)
			}
			if tt.logType != "" && result["type"] != tt.logType {
				t.Fatalf("type: got %v, want %v", result["type"], tt.logType)
			}
			if tt.subtype != "" && result["subtype"] != tt.subtype {
				t.Fatalf("subtype: got %v, want %v", result["subtype"], tt.subtype)
			}
			if tt.action != "" && result["action"] != tt.action {
				t.Fatalf("action: got %v, want %v", result["action"], tt.action)
			}
			if tt.level != "" && result["level"] != tt.level {
				t.Fatalf("level: got %v, want %v", result["level"], tt.level)
			}
			if tt.message != "" && result["message"] != tt.message {
				t.Fatalf("message: got %v, want %v", result["message"], tt.message)
			}
			if tt.name == "unknown keys preserved" {
				if result["customkey"] != "customval" {
					t.Fatalf("expected customkey preserved, got %v", result["customkey"])
				}
			}
		})
	}
}
