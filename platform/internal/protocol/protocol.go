package protocol

import (
	"encoding/binary"
	"fmt"
)

// Message types
const (
	MessageTypeCommand uint8 = 0x01
	MessageTypeEvent   uint8 = 0x02
)

// Action codes
const (
	ActionStart uint8 = 1
	ActionStop  uint8 = 2
)

// State codes
const (
	StateIdle        uint8 = 0
	StatePreheating  uint8 = 1
	StateBaking      uint8 = 2
	StateCoolingDown uint8 = 3
)

// CRC-16/USB parameters
const (
	crcPoly   uint16 = 0x8005
	crcInit   uint16 = 0xFFFF
	crcXorOut uint16 = 0xFFFF
)

// CommandPayload represents a command sent to the oven
type CommandPayload struct {
	Action      uint8
	Temperature int16
	Duration    int16
}

// EventPayload represents an event sent from the oven
type EventPayload struct {
	State                 uint8
	CurrentTemperature    int16
	RemainingTime         int16
	ProgrammedDuration    int16
	ProgrammedTemperature int16
}

// reflectByte reverses the bits in a byte
func reflectByte(b uint8) uint8 {
	result := uint8(0)
	for i := 0; i < 8; i++ {
		if b&(1<<i) != 0 {
			result |= 1 << (7 - i)
		}
	}
	return result
}

// reflectUint16 reverses the bits in a uint16
func reflectUint16(val uint16) uint16 {
	result := uint16(0)
	for i := 0; i < 16; i++ {
		if val&(1<<i) != 0 {
			result |= 1 << (15 - i)
		}
	}
	return result
}

// calculateCRC calculates CRC-16/USB checksum
func calculateCRC(data []byte) uint16 {
	crc := crcInit

	for _, b := range data {
		reflected := reflectByte(b)
		crc ^= uint16(reflected) << 8

		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ crcPoly
			} else {
				crc <<= 1
			}
		}
	}

	crc = reflectUint16(crc)
	crc ^= crcXorOut

	return crc
}

// MarshallCommandFrame creates a binary command frame
func MarshallCommandFrame(cmd *CommandPayload) ([]byte, error) {
	payloadSize := uint16(5)                  // 1 byte action + 2 bytes temp + 2 bytes duration
	frameSize := 1 + 2 + int(payloadSize) + 2 // msg_type + size + payload + crc

	frame := make([]byte, frameSize)
	offset := 0

	frame[offset] = MessageTypeCommand
	offset++

	binary.LittleEndian.PutUint16(frame[offset:], payloadSize)
	offset += 2

	frame[offset] = cmd.Action
	offset++
	binary.LittleEndian.PutUint16(frame[offset:], uint16(cmd.Temperature))
	offset += 2
	binary.LittleEndian.PutUint16(frame[offset:], uint16(cmd.Duration))
	offset += 2

	crc := calculateCRC(frame[:offset])
	binary.LittleEndian.PutUint16(frame[offset:], crc)

	return frame, nil
}

// UnmarshallEventFrame parses a binary event frame
func UnmarshallEventFrame(frame []byte) (*EventPayload, error) {
	if len(frame) < 5 { // Minimum: msg_type(1) + size(2) + crc(2)
		return nil, fmt.Errorf("frame too short: %d bytes", len(frame))
	}

	offset := 0

	// Message type
	msgType := frame[offset]
	if msgType != MessageTypeEvent {
		return nil, fmt.Errorf("invalid message type: expected 0x%02X, got 0x%02X", MessageTypeEvent, msgType)
	}
	offset++

	// Payload size
	payloadSize := binary.LittleEndian.Uint16(frame[offset:])
	offset += 2

	if len(frame) < int(3+payloadSize+2) {
		return nil, fmt.Errorf("frame too short for payload size %d", payloadSize)
	}

	// Verify CRC
	expectedCRC := binary.LittleEndian.Uint16(frame[3+payloadSize:])
	calculatedCRC := calculateCRC(frame[:3+payloadSize])
	if expectedCRC != calculatedCRC {
		return nil, fmt.Errorf("CRC mismatch: expected 0x%04X, got 0x%04X", expectedCRC, calculatedCRC)
	}

	// Parse payload
	event := &EventPayload{
		State:                 frame[offset],
		CurrentTemperature:    int16(binary.LittleEndian.Uint16(frame[offset+1:])),
		RemainingTime:         int16(binary.LittleEndian.Uint16(frame[offset+3:])),
		ProgrammedDuration:    int16(binary.LittleEndian.Uint16(frame[offset+5:])),
		ProgrammedTemperature: int16(binary.LittleEndian.Uint16(frame[offset+7:])),
	}

	return event, nil
}

// StateToString converts a state code to a string
func StateToString(state uint8) string {
	switch state {
	case StateIdle:
		return "IDLE"
	case StatePreheating:
		return "PREHEATING"
	case StateBaking:
		return "BAKING"
	case StateCoolingDown:
		return "COOLING_DOWN"
	default:
		return "UNKNOWN"
	}
}

// ActionToString converts an action code to a string
func ActionToString(action uint8) string {
	switch action {
	case ActionStart:
		return "START"
	case ActionStop:
		return "STOP"
	default:
		return "UNKNOWN"
	}
}
