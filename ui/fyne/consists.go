package fyneui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/Dylan-M/Go_DCC_Ex_Driver/dccex/capabilities"
	th "github.com/Dylan-M/Go_DCC_Ex_Driver/throttle"
)

func (v *View) consistTab() fyne.CanvasObject {
	v.consistStatus = wrappedLabel(consistDiscoveryText(th.State{}))
	return container.NewVScroll(container.NewVBox(
		widget.NewLabelWithStyle("Consist Builder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		v.consistStatus,
		widget.NewSeparator(),
		wrappedLabel("Consist editing and intelligent front/rear lighting are not available yet. This tab does not send commands or change existing consists."),
	))
}

func consistDiscoveryText(s th.State) string {
	if !s.Connected {
		return "Connect to a command station. Capability discovery is not configured in this build."
	}
	switch s.Discovery.Status {
	case capabilities.NotRequested:
		return "Capability discovery is not configured in this build. No discovery query has been sent."
	case capabilities.Pending:
		return "Waiting for the command station's capability response."
	case capabilities.Available:
		return "Capabilities received. Consist behavior verification and builder implementation remain pending."
	case capabilities.Unsupported:
		return "The command station rejected the capability query (<X>). Custom enhancements are not enabled."
	case capabilities.Inconclusive:
		return "Capability discovery timed out. Support is unknown; custom enhancements are not enabled."
	case capabilities.InvalidResponse:
		return "The capability response was invalid. Custom enhancements are not enabled."
	default:
		return "Capability support is unknown. Custom enhancements are not enabled."
	}
}
