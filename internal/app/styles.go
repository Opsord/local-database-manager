package app

import "github.com/charmbracelet/lipgloss"

var (
	// Night Owl Color Palette (aligned with OpenCode nightowl.json)
	BgDark         = lipgloss.Color("#011627")
	BgSurface      = lipgloss.Color("#0b253a")
	BgElement      = lipgloss.Color("#0d2137")
	FgText         = lipgloss.Color("#D6DEEB")
	PrimaryColor   = lipgloss.Color("#82AAFF")
	SecondaryColor = lipgloss.Color("#ADDB67")
	AccentColor    = lipgloss.Color("#C792EA")
	TealColor      = lipgloss.Color("#7FDBCA")
	WarningColor   = lipgloss.Color("#ECC48D")
	ErrorColor     = lipgloss.Color("#EF5350")
	MutedColor     = lipgloss.Color("#5F7E97")
	BorderColor    = lipgloss.Color("#1D3B53")
	BorderActive   = lipgloss.Color("#82AAFF")
	SelectedBg     = lipgloss.Color("#1D3B53")
	HeaderBg       = BgSurface

	HeaderStyle = lipgloss.NewStyle().
			Background(HeaderBg).
			Foreground(FgText).
			Padding(0, 1)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			BorderBackground(BgSurface).
			Background(BgSurface).
			Padding(0, 1)

	ActivePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderActive).
				BorderBackground(BgSurface).
				Background(BgSurface).
				Padding(0, 1)

	// Inner text must set Background(BgSurface). Lip Gloss emits SGR reset
	// when only foreground is set, which punches black holes through the panel.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor).
			Background(BgSurface)

	SubTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(AccentColor).
			Background(BgSurface)

	SelectedItemStyle = lipgloss.NewStyle().
				Background(SelectedBg).
				Bold(true).
				Foreground(FgText)

	NormalItemStyle = lipgloss.NewStyle().
			Foreground(FgText).
			Background(BgSurface)

	RunningStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true).
			Background(BgSurface)

	StartingStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true).
			Background(BgSurface)

	StoppedStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true).
			Background(BgSurface)

	UnknownStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Bold(true).
			Background(BgSurface)

	LabelStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Bold(true).
			Width(14).
			Background(BgSurface)

	ValueStyle = lipgloss.NewStyle().
			Foreground(FgText).
			Background(BgSurface)

	ValueHighlightStyle = lipgloss.NewStyle().
				Foreground(TealColor).
				Bold(true).
				Background(BgSurface)

	URIBoxStyle = lipgloss.NewStyle().
			Foreground(TealColor).
			Background(BgSurface)

	CLIBoxStyle = lipgloss.NewStyle().
			Foreground(WarningColor).
			Background(BgSurface)

	MutedStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Background(BgSurface)

	SeparatorStyle = lipgloss.NewStyle().
			Foreground(BorderColor).
			Background(BgSurface)

	EnvBoxStyle = lipgloss.NewStyle().
			Background(BgSurface).
			Foreground(FgText)

	InputBoxStyle = lipgloss.NewStyle().
			Background(BgSurface).
			Foreground(FgText)

	FilterBoxStyle = lipgloss.NewStyle().
			Foreground(TealColor).
			Background(BgSurface)

	LogAreaStyle = lipgloss.NewStyle().
			Background(BgSurface).
			Foreground(FgText)

	StatusBarStyle = lipgloss.NewStyle().
			Foreground(FgText).
			Background(BgSurface).
			Padding(0, 1)

	KeyStyle = lipgloss.NewStyle().
			Foreground(PrimaryColor).
			Bold(true).
			Background(BgSurface)

	KeyDescStyle = lipgloss.NewStyle().
			Foreground(FgText).
			Background(BgSurface)

	BadgeStyle = lipgloss.NewStyle().
			Background(SelectedBg).
			Padding(0, 1)

	BadgeOnlineStyle = BadgeStyle.Copy().
				Foreground(SecondaryColor)

	BadgeOfflineStyle = BadgeStyle.Copy().
				Foreground(ErrorColor)

	BadgeUnknownStyle = BadgeStyle.Copy().
				Foreground(MutedColor)
)
