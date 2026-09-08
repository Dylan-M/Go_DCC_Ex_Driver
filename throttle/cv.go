package throttle

import "fmt"

var cvNames = map[int]string{
	1: "Primary (short) address", 2: "Vstart -- motor start voltage", 3: "Acceleration rate", 4: "Deceleration rate", 5: "Vhigh -- top speed voltage", 6: "Vmid -- mid speed voltage",
	7: "Manufacturer version, read-only", 8: "Manufacturer ID -- writing it resets many decoders", 17: "Extended address high byte", 18: "Extended address low byte",
	19: "Consist address", 21: "Consist functions F1-F8", 22: "Consist functions FL, F9-F12", 23: "Acceleration adjustment", 24: "Deceleration adjustment",
	28: "RailCom configuration", 29: "Configuration data #1", 30: "Error information", 65: "Kick start", 66: "Forward trim", 95: "Reverse trim", 105: "User ID #1", 106: "User ID #2",
}

func CVName(cv int) string {
	if name, ok := cvNames[cv]; ok {
		return name
	}
	if cv >= 33 && cv <= 46 {
		return "Function output mapping"
	}
	if cv >= 67 && cv <= 94 {
		return fmt.Sprintf("Speed table entry %d/28", cv-66)
	}
	if cv >= 112 && cv <= 256 {
		return "Manufacturer-specific"
	}
	return ""
}
func CVDescription(cv int) string {
	if name := CVName(cv); name != "" {
		return fmt.Sprintf("CV %d (%s)", cv, name)
	}
	return fmt.Sprintf("CV %d", cv)
}
