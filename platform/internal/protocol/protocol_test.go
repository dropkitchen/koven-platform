package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestReflectByte tests the bit reflection function for bytes
func TestReflectByte(t *testing.T) {
	tests := []struct {
		name     string
		input    uint8
		expected uint8
	}{
		{"zero", 0x00, 0x00},
		{"all ones", 0xFF, 0xFF},
		{"alternating pattern 1", 0xAA, 0x55},
		{"alternating pattern 2", 0x55, 0xAA},
		{"single bit low", 0x01, 0x80},
		{"single bit high", 0x80, 0x01},
		{"specific pattern", 0xF0, 0x0F},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reflectByte(tt.input)
			if result != tt.expected {
				t.Errorf("reflectByte(0x%02X) = 0x%02X, want 0x%02X", tt.input, result, tt.expected)
			}
		})
	}
}

// TestReflectUint16 tests the bit reflection function for uint16
func TestReflectUint16(t *testing.T) {
	tests := []struct {
		name     string
		input    uint16
		expected uint16
	}{
		{"zero", 0x0000, 0x0000},
		{"all ones", 0xFFFF, 0xFFFF},
		{"alternating pattern 1", 0xAAAA, 0x5555},
		{"alternating pattern 2", 0x5555, 0xAAAA},
		{"single bit low", 0x0001, 0x8000},
		{"single bit high", 0x8000, 0x0001},
		{"specific pattern", 0xF0F0, 0x0F0F},
		{"byte boundary", 0xFF00, 0x00FF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reflectUint16(tt.input)
			if result != tt.expected {
				t.Errorf("reflectUint16(0x%04X) = 0x%04X, want 0x%04X", tt.input, result, tt.expected)
			}
		})
	}
}

// TestCalculateCRC tests the CRC-16/USB calculation
func TestCalculateCRC(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected uint16
	}{
		{
			name:     "empty data",
			data:     []byte{},
			expected: 0x0000,
		},
		{
			name:     "single zero byte",
			data:     []byte{0x00},
			expected: 0xBF40,
		},
		{
			name:     "single 0xFF byte",
			data:     []byte{0xFF},
			expected: 0xFF00,
		},
		{
			name:     "simple sequence",
			data:     []byte{0x01, 0x02, 0x03},
			expected: 0x9E9E,
		},
		{
			name:     "message type command",
			data:     []byte{MessageTypeCommand},
			expected: 0x7F81,
		},
		{
			name:     "message type event",
			data:     []byte{MessageTypeEvent},
			expected: 0x7EC1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateCRC(tt.data)
			if result != tt.expected {
				t.Errorf("calculateCRC(%v) = 0x%04X, want 0x%04X", tt.data, result, tt.expected)
			}
		})
	}
}

// TestMarshallCommandFrame tests command frame creation
func TestMarshallCommandFrame(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *CommandPayload
		expected []byte
		wantErr  bool
	}{
		{
			name: "START command with positive values",
			cmd: &CommandPayload{
				Action:      ActionStart,
				Temperature: 200,
				Duration:    3600,
			},
			// Frame structure:
			// [0]    = 0x01 (MessageTypeCommand)
			// [1-2]  = 0x05 0x00 (payload size = 5, little-endian)
			// [3]    = 0x01 (ActionStart)
			// [4-5]  = 0xC8 0x00 (temperature = 200, little-endian)
			// [6-7]  = 0x10 0x0E (duration = 3600, little-endian)
			// [8-9]  = CRC-16/USB (calculated over bytes 0-7)
			expected: []byte{0x01, 0x05, 0x00, 0x01, 0xC8, 0x00, 0x10, 0x0E, 0xA4, 0x9C},
			wantErr:  false,
		},
		{
			name: "STOP command",
			cmd: &CommandPayload{
				Action:      ActionStop,
				Temperature: 0,
				Duration:    0,
			},
			// Frame structure:
			// [0]    = 0x01 (MessageTypeCommand)
			// [1-2]  = 0x05 0x00 (payload size = 5)
			// [3]    = 0x02 (ActionStop)
			// [4-5]  = 0x00 0x00 (temperature = 0)
			// [6-7]  = 0x00 0x00 (duration = 0)
			// [8-9]  = CRC-16/USB
			expected: []byte{0x01, 0x05, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x52, 0xF8},
			wantErr:  false,
		},
		{
			name: "negative temperature",
			cmd: &CommandPayload{
				Action:      ActionStart,
				Temperature: -10,
				Duration:    100,
			},
			// Frame structure:
			// [0]    = 0x01 (MessageTypeCommand)
			// [1-2]  = 0x05 0x00 (payload size = 5)
			// [3]    = 0x01 (ActionStart)
			// [4-5]  = 0xF6 0xFF (temperature = -10, little-endian two's complement)
			// [6-7]  = 0x64 0x00 (duration = 100)
			// [8-9]  = CRC-16/USB
			expected: []byte{0x01, 0x05, 0x00, 0x01, 0xF6, 0xFF, 0x64, 0x00, 0x3F, 0x80},
			wantErr:  false,
		},
		{
			name: "maximum positive values",
			cmd: &CommandPayload{
				Action:      ActionStart,
				Temperature: 32767,
				Duration:    32767,
			},
			// Frame structure:
			// [0]    = 0x01 (MessageTypeCommand)
			// [1-2]  = 0x05 0x00 (payload size = 5)
			// [3]    = 0x01 (ActionStart)
			// [4-5]  = 0xFF 0x7F (temperature = 32767)
			// [6-7]  = 0xFF 0x7F (duration = 32767)
			// [8-9]  = CRC-16/USB
			expected: []byte{0x01, 0x05, 0x00, 0x01, 0xFF, 0x7F, 0xFF, 0x7F, 0x17, 0x24},
			wantErr:  false,
		},
		{
			name: "zero values",
			cmd: &CommandPayload{
				Action:      ActionStart,
				Temperature: 0,
				Duration:    0,
			},
			// Frame structure:
			// [0]    = 0x01 (MessageTypeCommand)
			// [1-2]  = 0x05 0x00 (payload size = 5)
			// [3]    = 0x01 (ActionStart)
			// [4-5]  = 0x00 0x00 (temperature = 0)
			// [6-7]  = 0x00 0x00 (duration = 0)
			// [8-9]  = CRC-16/USB
			expected: []byte{0x01, 0x05, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x16, 0xF8},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := MarshallCommandFrame(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshallCommandFrame() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if !bytes.Equal(frame, tt.expected) {
					t.Errorf("MarshallCommandFrame() frame mismatch\ngot:  %v\nwant: %v", frame, tt.expected)
					// Also show hex for easier debugging
					t.Errorf("got (hex):  % X", frame)
					t.Errorf("want (hex): % X", tt.expected)
				}
			}
		})
	}
}

// TestUnmarshallEventFrame tests event frame parsing
func TestUnmarshallEventFrame(t *testing.T) {
	tests := []struct {
		name    string
		frame   []byte
		want    *EventPayload
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid IDLE state event",
			// Frame structure:
			// [0]     = 0x02 (MessageTypeEvent)
			// [1-2]   = 0x09 0x00 (payload size = 9, little-endian)
			// [3]     = 0x00 (StateIdle)
			// [4-5]   = 0x19 0x00 (current temp = 25, little-endian)
			// [6-7]   = 0x00 0x00 (remaining time = 0)
			// [8-9]   = 0x00 0x00 (programmed duration = 0)
			// [10-11] = 0x00 0x00 (programmed temp = 0)
			// [12-13] = CRC-16/USB (calculated over bytes 0-11)
			frame: []byte{0x02, 0x09, 0x00, 0x00, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x8D, 0xC0},
			want: &EventPayload{
				State:                 StateIdle,
				CurrentTemperature:    25,
				RemainingTime:         0,
				ProgrammedDuration:    0,
				ProgrammedTemperature: 0,
			},
			wantErr: false,
		},
		{
			name: "valid BAKING state event",
			// Frame structure:
			// [0]     = 0x02 (MessageTypeEvent)
			// [1-2]   = 0x09 0x00 (payload size = 9)
			// [3]     = 0x02 (StateBaking)
			// [4-5]   = 0xB4 0x00 (current temp = 180)
			// [6-7]   = 0x08 0x07 (remaining time = 1800)
			// [8-9]   = 0x10 0x0E (programmed duration = 3600)
			// [10-11] = 0xC8 0x00 (programmed temp = 200)
			// [12-13] = CRC-16/USB
			frame: []byte{0x02, 0x09, 0x00, 0x02, 0xB4, 0x00, 0x08, 0x07, 0x10, 0x0E, 0xC8, 0x00, 0xD9, 0x0A},
			want: &EventPayload{
				State:                 StateBaking,
				CurrentTemperature:    180,
				RemainingTime:         1800,
				ProgrammedDuration:    3600,
				ProgrammedTemperature: 200,
			},
			wantErr: false,
		},
		{
			name: "valid PREHEATING state event",
			// Frame structure:
			// [0]     = 0x02 (MessageTypeEvent)
			// [1-2]   = 0x09 0x00 (payload size = 9)
			// [3]     = 0x01 (StatePreheating)
			// [4-5]   = 0x64 0x00 (current temp = 100)
			// [6-7]   = 0x10 0x0E (remaining time = 3600)
			// [8-9]   = 0x10 0x0E (programmed duration = 3600)
			// [10-11] = 0xDC 0x00 (programmed temp = 220)
			// [12-13] = CRC-16/USB
			frame: []byte{0x02, 0x09, 0x00, 0x01, 0x64, 0x00, 0x10, 0x0E, 0x10, 0x0E, 0xDC, 0x00, 0x10, 0x7F},
			want: &EventPayload{
				State:                 StatePreheating,
				CurrentTemperature:    100,
				RemainingTime:         3600,
				ProgrammedDuration:    3600,
				ProgrammedTemperature: 220,
			},
			wantErr: false,
		},
		{
			name: "valid COOLING_DOWN state event",
			// Frame structure:
			// [0]     = 0x02 (MessageTypeEvent)
			// [1-2]   = 0x09 0x00 (payload size = 9)
			// [3]     = 0x03 (StateCoolingDown)
			// [4-5]   = 0x50 0x00 (current temp = 80)
			// [6-7]   = 0x00 0x00 (remaining time = 0)
			// [8-9]   = 0x10 0x0E (programmed duration = 3600)
			// [10-11] = 0xC8 0x00 (programmed temp = 200)
			// [12-13] = CRC-16/USB
			frame: []byte{0x02, 0x09, 0x00, 0x03, 0x50, 0x00, 0x00, 0x00, 0x10, 0x0E, 0xC8, 0x00, 0x6F, 0xA9},
			want: &EventPayload{
				State:                 StateCoolingDown,
				CurrentTemperature:    80,
				RemainingTime:         0,
				ProgrammedDuration:    3600,
				ProgrammedTemperature: 200,
			},
			wantErr: false,
		},
		{
			name:    "frame too short",
			frame:   []byte{0x01, 0x02},
			want:    nil,
			wantErr: true,
			errMsg:  "frame too short",
		},
		{
			name:    "empty frame",
			frame:   []byte{},
			want:    nil,
			wantErr: true,
			errMsg:  "frame too short",
		},
		{
			name:    "invalid message type",
			frame:   []byte{0xFF, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			want:    nil,
			wantErr: true,
			errMsg:  "invalid message type",
		},
		{
			name: "CRC mismatch",
			// Same as IDLE state but with intentionally wrong CRC
			// [0]     = 0x02 (MessageTypeEvent)
			// [1-2]   = 0x09 0x00 (payload size = 9)
			// [3]     = 0x00 (StateIdle)
			// [4-5]   = 0x19 0x00 (current temp = 25)
			// [6-7]   = 0x00 0x00 (remaining time = 0)
			// [8-9]   = 0x00 0x00 (programmed duration = 0)
			// [10-11] = 0x00 0x00 (programmed temp = 0)
			// [12-13] = 0xDE 0xAD (WRONG CRC - should be 0x8D 0xC0)
			frame:   []byte{0x02, 0x09, 0x00, 0x00, 0x19, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xDE, 0xAD},
			want:    nil,
			wantErr: true,
			errMsg:  "CRC mismatch",
		},
		{
			name: "payload size mismatch - frame too short",
			frame: func() []byte {
				frame := make([]byte, 10)
				frame[0] = MessageTypeEvent
				binary.LittleEndian.PutUint16(frame[1:], 20) // claims 20 bytes but frame is shorter
				return frame
			}(),
			want:    nil,
			wantErr: true,
			errMsg:  "frame too short for payload size",
		},
		{
			name: "negative temperature values",
			// Frame structure:
			// [0]     = 0x02 (MessageTypeEvent)
			// [1-2]   = 0x09 0x00 (payload size = 9)
			// [3]     = 0x00 (StateIdle)
			// [4-5]   = 0xFB 0xFF (current temp = -5, two's complement)
			// [6-7]   = 0x00 0x00 (remaining time = 0)
			// [8-9]   = 0x00 0x00 (programmed duration = 0)
			// [10-11] = 0xF6 0xFF (programmed temp = -10, two's complement)
			// [12-13] = CRC-16/USB
			frame: []byte{0x02, 0x09, 0x00, 0x00, 0xFB, 0xFF, 0x00, 0x00, 0x00, 0x00, 0xF6, 0xFF, 0x0A, 0xBE},
			want: &EventPayload{
				State:                 StateIdle,
				CurrentTemperature:    -5,
				RemainingTime:         0,
				ProgrammedDuration:    0,
				ProgrammedTemperature: -10,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UnmarshallEventFrame(tt.frame)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshallEventFrame() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !bytes.Contains([]byte(err.Error()), []byte(tt.errMsg)) {
					t.Errorf("UnmarshallEventFrame() error message = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}
			if !tt.wantErr && got != nil && tt.want != nil {
				if got.State != tt.want.State {
					t.Errorf("State = %d, want %d", got.State, tt.want.State)
				}
				if got.CurrentTemperature != tt.want.CurrentTemperature {
					t.Errorf("CurrentTemperature = %d, want %d", got.CurrentTemperature, tt.want.CurrentTemperature)
				}
				if got.RemainingTime != tt.want.RemainingTime {
					t.Errorf("RemainingTime = %d, want %d", got.RemainingTime, tt.want.RemainingTime)
				}
				if got.ProgrammedDuration != tt.want.ProgrammedDuration {
					t.Errorf("ProgrammedDuration = %d, want %d", got.ProgrammedDuration, tt.want.ProgrammedDuration)
				}
				if got.ProgrammedTemperature != tt.want.ProgrammedTemperature {
					t.Errorf("ProgrammedTemperature = %d, want %d", got.ProgrammedTemperature, tt.want.ProgrammedTemperature)
				}
			}
		})
	}
}

// TestStateToString tests state code to string conversion
func TestStateToString(t *testing.T) {
	tests := []struct {
		name  string
		state uint8
		want  string
	}{
		{"idle state", StateIdle, "IDLE"},
		{"preheating state", StatePreheating, "PREHEATING"},
		{"baking state", StateBaking, "BAKING"},
		{"cooling down state", StateCoolingDown, "COOLING_DOWN"},
		{"unknown state", 99, "UNKNOWN"},
		{"invalid state", 255, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StateToString(tt.state)
			if got != tt.want {
				t.Errorf("StateToString(%d) = %s, want %s", tt.state, got, tt.want)
			}
		})
	}
}

// TestActionToString tests action code to string conversion
func TestActionToString(t *testing.T) {
	tests := []struct {
		name   string
		action uint8
		want   string
	}{
		{"start action", ActionStart, "START"},
		{"stop action", ActionStop, "STOP"},
		{"unknown action", 99, "UNKNOWN"},
		{"invalid action", 255, "UNKNOWN"},
		{"zero action", 0, "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ActionToString(tt.action)
			if got != tt.want {
				t.Errorf("ActionToString(%d) = %s, want %s", tt.action, got, tt.want)
			}
		})
	}
}
