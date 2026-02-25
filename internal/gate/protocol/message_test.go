package protocol

import (
	"testing"
)

func TestNew(t *testing.T) {
	m := New(1, 2, []byte("data"))
	if m == nil || m.Head == nil {
		t.Fatal("New should return non-nil Message with Head")
	}
	if m.Cmd != 1 || m.Act != 2 {
		t.Errorf("Cmd/Act want 1/2, got %d/%d", m.Cmd, m.Act)
	}
	if string(m.Data) != "data" {
		t.Errorf("Data want 'data', got %q", m.Data)
	}
	if m.Len != 0 || m.Error != 0 || m.Index != 0 {
		t.Errorf("Head fields should be zero, got Len=%d Error=%d Index=%d", m.Len, m.Error, m.Index)
	}
}

func TestNewWithData(t *testing.T) {
	m := NewWithData([]byte("x"))
	if m == nil || m.Head == nil {
		t.Fatal("NewWithData should return non-nil Message")
	}
	if string(m.Data) != "x" {
		t.Errorf("Data want 'x', got %q", m.Data)
	}
}

func TestNewErr(t *testing.T) {
	m := NewErr(3, 4, 500)
	if m == nil {
		t.Fatal("NewErr should not return nil")
	}
	if m.Cmd != 3 || m.Act != 4 {
		t.Errorf("Cmd/Act want 3/4, got %d/%d", m.Cmd, m.Act)
	}
	if m.Error != 500 {
		t.Errorf("Error want 500, got %d", m.Error)
	}
}

func TestMessage_Copy(t *testing.T) {
	dst := New(0, 0, nil)
	old := New(5, 6, nil)
	old.Index = 100
	dst.Copy(old)
	if dst.Cmd != 5 || dst.Act != 6 || dst.Index != 100 {
		t.Errorf("Copy: want Cmd=5 Act=6 Index=100, got Cmd=%d Act=%d Index=%d", dst.Cmd, dst.Act, dst.Index)
	}
	dst.Copy(nil) // 不应 panic
}

func TestMessage_ID(t *testing.T) {
	m := New(0x01, 0x02, nil)
	id := m.ID()
	want := uint16(0x01)<<8 + 0x02
	if id != want {
		t.Errorf("ID() want %d, got %d", want, id)
	}
}

func TestCmdAct(t *testing.T) {
	tests := []struct {
		cmd, act uint8
		want     uint16
	}{
		{0, 0, 0},
		{1, 0, 256},
		{0, 1, 1},
		{1, 2, 258},
		{0xFF, 0xFF, 0xFFFF},
	}
	for _, tt := range tests {
		got := CmdAct(tt.cmd, tt.act)
		if got != tt.want {
			t.Errorf("CmdAct(%d,%d) want %d, got %d", tt.cmd, tt.act, tt.want, got)
		}
	}
}

func TestParseId(t *testing.T) {
	cmd, act := ParseId(0x0102)
	if cmd != 1 || act != 2 {
		t.Errorf("ParseId(0x0102) want cmd=1 act=2, got cmd=%d act=%d", cmd, act)
	}
	cmd, act = ParseId(0)
	if cmd != 0 || act != 0 {
		t.Errorf("ParseId(0) want 0,0, got %d,%d", cmd, act)
	}
}

func TestHeadLen(t *testing.T) {
	if HeadLen != 13 {
		t.Errorf("HeadLen want 13, got %d", HeadLen)
	}
}
