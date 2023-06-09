// Package sdk is a software development kit for building blockchain applications.
// File sdk/html5.go - HTML5 related functions for the SDK
package sdk

import (
	"strings"
)

// HTML5 is a struct to hold the HTML5 settings & methods
type HTML5 struct {
	EndableUsers bool
	initialized  bool
}

var (
	// HTML is the HTML5 object
	HTML *HTML5
)

// init initializes the HTML5 object
func init() {
	HTML = &HTML5{initialized: true}
}

// RenderPage returns string content representing a valid HTML5 page
func (h *HTML5) RenderPage(title, content string) string {
	return strings.Join([]string{"<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\" />\n<title>", BlockchainName, "(", BlockchainSymbol, ")</title>\n<link rel=\"shortcut icon\" href=\"favicon.ico\" type=\"image/x-icon\">\n<style>\nhtml,body {height:100%;width:100%;margin:0;}\nbody , body {display:flex;}\n#content,form {margin:auto;}\n</style>\n</head>\n<body>\n<div id=\"content\">", content, "</div>\n</body>\n</html>\n"}, "")
}

// RenderPageHeader returns string content representing a valid HTML5 page header
func (h *HTML5) RenderPageHeader() string {
	return "{PAGE_HEADER}"
}

// RenderFormLogin returns string content representing a valid HTML5 login form
func (h *HTML5) RenderFormLogin() string {
	return "<form id=\"form_login\" action=\"/v0/user/login\" method=\"post\">\n<h1>Test Login Page</h1><p><input type=\"text\" id=\"email\" name=\"email\" required placeholder=\"account email\" /></p>\n<p><input type=\"password\" id=\"password\" name=\"password\" required placeholder=\"password\" /></p>\n<p><button id=\"submitbutton\" type=\"submit\">Login</button></p>\n</form>\n"
}

// RenderPageLogin returns string content representing a valid HTML5 page with the login form centered on the page
func (h *HTML5) RenderPageLogin() string {
	return h.RenderPage("Member Login", h.RenderFormLogin())
}

// RenderPageNotImplemented returns string content representing a valid HTML5 page with a message that the page is not yet implemented
func (h *HTML5) RenderPageNotImplemented(name string) string {
	return h.RenderPage("Forgot Password", "<h1>"+name+" Not yet implemented</h1>")
}
