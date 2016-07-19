package server

import (
	"log"
	"net/http"
	"net/url"
	"text/template"

	"github.com/ericchiang/poke/storage"
)

type connectorInfo struct {
	DisplayName string
	URL         string
}

var loginTmpl = template.Must(template.New("login-template").Parse(`<html>
<head></head>
<body>
<p>Login options</p>
{{ range $i, $connector := .Connectors }}
<a href="{{ $connector.URL }}?{{ $.URLQuery }}">{{ $connector.DisplayName }}</a>
{{ end }}
</body>
</html>`))

func renderLoginOptions(w http.ResponseWriter, connectors []connectorInfo, query url.Values) {
	data := struct {
		Connectors []connectorInfo
		URLQuery   string
	}{connectors, query.Encode()}
	renderTemplate(w, loginTmpl, data)
}

var passwordTmpl = template.Must(template.New("password-template").Parse(`<html>
<body>
<p>Login through LDAP</p>
<form action="{{ .Callback }}" method="POST">
Login: <input type="text" name="login"/><br/>
Password: <input type="password" name="password"/><br/>
<input type="hidden" name="state" value="{{ .State }}"/>
<input type="submit"/>
{{ if .Message }}
<p>Error: {{ .Message }}</p>
{{ end }}
</form>
</body>
</html>`))

func renderPasswordTmpl(w http.ResponseWriter, state, callback, message string) {
	data := struct {
		State    string
		Callback string
		Message  string
	}{state, callback, message}
	renderTemplate(w, passwordTmpl, data)
}

var approvalTmpl = template.Must(template.New("approval-template").Parse(`<html>
<body>
<p>User: {{ .User }}</p>
<p>Client: {{ .ClientName }}</p>
<form method="POST">
<input type="hidden" name="state" value="{{ .State }}"/>
<button name="subject" type="submit" value="approve">Approve</button>
<button name="subject" type="submit" value="reject">Reject</button>
</form>
</body>
</html>`))

func renderApprovalTmpl(w http.ResponseWriter, state string, identity storage.Identity, client storage.Client, scopes []string) {
	data := struct {
		User       string
		ClientName string
		State      string
	}{identity.Email, client.Name, state}
	renderTemplate(w, approvalTmpl, data)
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, data interface{}) {
	err := tmpl.Execute(w, data)
	if err == nil {
		return
	}

	switch err := err.(type) {
	case template.ExecError:
		// An ExecError guarentees that Execute has not written to the underlying reader.
		log.Printf("Error rendering template %s: %s", tmpl.Name(), err)

		// TODO(ericchiang): replace with better internal server error.
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	default:
		// An error with the underlying write, such as the connection being
		// dropped. Ignore for now.
	}
}
