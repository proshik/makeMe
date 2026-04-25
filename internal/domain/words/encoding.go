package words

// ModeCode is the short callback-data form of a Mode (4 chars).
type ModeCode string

const (
	codeEnRuMC     ModeCode = "enrm"
	codeRuEnMC     ModeCode = "renm"
	codeEnRuTyping ModeCode = "enrt"
	codeRuEnTyping ModeCode = "rent"
)

// EncodeMode returns the short callback-data form for a Mode.
func EncodeMode(m Mode) ModeCode {
	switch m {
	case ModeEnRuMC:
		return codeEnRuMC
	case ModeRuEnMC:
		return codeRuEnMC
	case ModeEnRuTyping:
		return codeEnRuTyping
	case ModeRuEnTyping:
		return codeRuEnTyping
	}
	return ""
}

// DecodeMode reverses EncodeMode. Returns ("", false) for unknown codes.
func DecodeMode(code string) (Mode, bool) {
	switch ModeCode(code) {
	case codeEnRuMC:
		return ModeEnRuMC, true
	case codeRuEnMC:
		return ModeRuEnMC, true
	case codeEnRuTyping:
		return ModeEnRuTyping, true
	case codeRuEnTyping:
		return ModeRuEnTyping, true
	}
	return "", false
}
