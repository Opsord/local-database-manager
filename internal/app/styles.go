package app

import "github.com/charmbracelet/lipgloss"

var (
	// Night Owl Color Palette
	BgDark         = lipgloss.Color("#011627")
	BgSurface      = lipgloss.Color("#0b2942")
	FgText         = lipgloss.Color("#D6DEEB")
	PrimaryColor   = lipgloss.Color("#82AAFF") // Functions / Blue Accent
	SecondaryColor = lipgloss.Color("#ADDB67") // Support / Neon Green
	AccentColor    = lipgloss.Color("#C792EA") // Keywords / Lavender Purple
	TealColor      = lipgloss.Color("#7FDBCA") // Variables / Cyan
	WarningColor   = lipgloss.Color("#ECC48D") // Numbers / Soft Gold
	ErrorColor     = lipgloss.Color("#EF5350") // Coral Red
	MutedColor     = lipgloss.Color("#5F7E97") // Search highlight / Slate
	BorderColor    = lipgloss.Color("#1D3B53") // Deep Navy Border
	BorderActive   = lipgloss.Color("#82AAFF") // Active Blue Border
	SelectedBg     = lipgloss.Color("#1D3B53") // Active Selection
	HeaderBg       = lipgloss.Color("#0b2942")

	// Box Styles
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(0, 1)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderActive).
				Padding(0, 1)

	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BgDark).
			Background(PrimaryColor).
			Padding(0, 1)

	SubTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor)

	// List Item Styles
	SelectedItemStyle = lipgloss.NewStyle().
				Background(SelectedBg).
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF"))

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(FgText)

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
			Foreground(FgText)

	ValueHighlightStyle = lipgloss.NewStyle().
				Foreground(TealColor).
				Bold(true)

	URIBoxStyle = lipgloss.NewStyle().
			Background(BgDark).
			Foreground(TealColor).
			Padding(0, 1)

	CLIBoxStyle = lipgloss.NewStyle().
			Background(BgDark).
			Foreground(WarningColor).
			Padding(0, 1)

	EnvBoxStyle = lipgloss.NewStyle().
			Background(BgDark).
			Foreground(FgText).
			Padding(0, 1)

	// Footer & Status Bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(FgText).
			Background(BgSurface).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true)

	KeyDescStyle = lipgloss.NewStyle().
			Foreground(FgText)

	BadgeStyle = lipgloss.NewStyle().
			Background(SelectedBg).
			Padding(0, 1)
)
