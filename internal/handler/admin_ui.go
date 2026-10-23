package handler

import _ "embed"

//go:embed admin.html
var adminPanelHTML string

func AdminPanelHTML() string {
	return adminPanelHTML
}
