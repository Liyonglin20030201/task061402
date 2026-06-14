package inspector

import (
	"testing"
)

func TestReplicationInspector_Name(t *testing.T) {
	insp := NewReplicationInspector()
	if insp.Name() != "replication" {
		t.Errorf("expected name 'replication', got %q", insp.Name())
	}
}

func TestParseLag(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{"nil", nil, 0},
		{"int64", int64(42), 42},
		{"bytes", []byte("100"), 100},
		{"string", "55", 55},
		{"invalid string", "abc", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLag(tt.input)
			if got != tt.expected {
				t.Errorf("parseLag(%v) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestByteToString(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{"nil", nil, ""},
		{"bytes", []byte("Yes"), "Yes"},
		{"string", "No", "No"},
		{"int", 42, "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := byteToString(tt.input)
			if got != tt.expected {
				t.Errorf("byteToString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseRedisInfo(t *testing.T) {
	info := `# Replication
role:master
connected_slaves:2
slave0:ip=10.0.0.1,port=6379,state=online,offset=12345,lag=0
slave1:ip=10.0.0.2,port=6379,state=online,offset=12300,lag=1
master_repl_offset:12345
`
	m := parseRedisInfo(info)
	if m["role"] != "master" {
		t.Errorf("expected role=master, got %q", m["role"])
	}
	if m["connected_slaves"] != "2" {
		t.Errorf("expected connected_slaves=2, got %q", m["connected_slaves"])
	}
	if m["master_repl_offset"] != "12345" {
		t.Errorf("expected master_repl_offset=12345, got %q", m["master_repl_offset"])
	}
}

func TestParseRedisSlaveOffset(t *testing.T) {
	tests := []struct {
		name         string
		slaveInfo    string
		masterOffset string
		expected     int
	}{
		{"no lag", "ip=10.0.0.1,port=6379,state=online,offset=12345,lag=0", "12345", 0},
		{"with lag", "ip=10.0.0.1,port=6379,state=online,offset=12000,lag=1", "12345", 345},
		{"slave ahead (impossible but safe)", "ip=10.0.0.1,port=6379,state=online,offset=13000,lag=0", "12345", 0},
		{"no offset field", "ip=10.0.0.1,port=6379,state=online", "12345", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRedisSlaveOffset(tt.slaveInfo, tt.masterOffset)
			if got != tt.expected {
				t.Errorf("parseRedisSlaveOffset() = %d, want %d", got, tt.expected)
			}
		})
	}
}
