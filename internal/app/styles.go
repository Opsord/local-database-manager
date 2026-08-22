package app

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#7D56F4")
	SecondaryColor = lipgloss.Color("#04B575")
	ErrorColor     = lipgloss.Color("#FF4757")
	WarningColor   = lipgloss.Color("#FFA502")
	MutedColor     = lipgloss.Color("#747D8C")
	BorderColor    = lipgloss.Color("#57606F")
	SelectedBg     = lipgloss.Color("#2F3542")
	HeaderBg       = lipgloss.Color("#5352ED")

	// Box Styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor).
				Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 1)

	// List Item Styles
	SelectedItemStyle = lipgloss.NewStyle().
				Background(SelectedBg).
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F1F2F6"))

	// Status Indicator Styles
	RunningStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	StartingStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	StoppedStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	UnknownStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	// Details Styles
	LabelStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Bold(true).
			Width(14)

	ValueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	URIBoxStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E272E")).
			Foreground(lipgloss.Color("#00D2D3")).
			Padding(0, 1)

	CLIBoxStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E272E")).
			Foreground(lipgloss.Color("#FFA502")).
			Padding(0, 1)

	EnvBoxStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#1E272E")).
			Foreground(lipgloss.Color("#A4B0BE")).
			Padding(0, 1)

	// Footer & Status Bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#2F3542")).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	KeyDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CED6E0"))
)
