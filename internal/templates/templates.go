package templates

import (
	"bytes"
	"fmt"
	"html/template"
)

const (
	passwordResetSubject   = "Password Reset Request"
	passwordChangedSubject = "Your password has been changed"
)

var (
	passwordResetTmpl   = template.Must(template.New("password_reset").Parse(passwordResetHTML))
	passwordChangedTmpl = template.Must(template.New("password_changed").Parse(passwordChangedHTML))
)

const passwordResetHTML = `<html>
    <body>
        <h1>Password Reset Request</h1>
        <p><b>Hello!</b> This is your password reset token: <code>{{.Token}}</code>.</p>
        <p>Thanks,<br>Minibank</p>
    </body>
</html>`

const passwordChangedHTML = `<html>
    <body>
        <h1>Password Changed!</h1>
        <p><b>Hello!</b> Your password has been changed.</p>
        <p>Thanks,<br>Minibank</p>
    </body>
</html>`

type PasswordResetData struct {
	Token string
}

func GetPasswordResetContent(token string) (string, string, error) {
	var body bytes.Buffer
	data := PasswordResetData{Token: token}
	if err := passwordResetTmpl.Execute(&body, data); err != nil {
		return "", "", fmt.Errorf("failed to execute password reset template: %w", err)
	}
	return passwordResetSubject, body.String(), nil
}

func GetPasswordChangedContent() (string, string, error) {
	var body bytes.Buffer
	if err := passwordChangedTmpl.Execute(&body, nil); err != nil {
		return "", "", fmt.Errorf("failed to execute password changed template: %w", err)
	}
	return passwordChangedSubject, body.String(), nil
}
