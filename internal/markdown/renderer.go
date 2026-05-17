package markdown

import (
	"bytes"
	"errors"
	"text/template"

	"github.com/nicolasperalta/silo2/internal/identity"
)

// renderData is the template input. It deliberately excludes any time-based
// fields so that rendering is deterministic and idempotent.
type renderData struct {
	Identity *identity.Identity
}

func Render(ident *identity.Identity) (map[string]string, error) {
	if ident == nil {
		return nil, errors.New("identity is nil")
	}
	data := renderData{Identity: ident}

	out := map[string]string{}

	var err error
	out["Identity.md"], err = renderOne(identityTemplate, data)
	if err != nil {
		return nil, err
	}
	out["Skills.md"], err = renderOne(skillsTemplate, data)
	if err != nil {
		return nil, err
	}
	out["Projects.md"], err = renderOne(projectsTemplate, data)
	if err != nil {
		return nil, err
	}
	out["Outputs.md"], err = renderOne(outputsTemplate, data)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func renderOne(tmpl string, data renderData) (string, error) {
	t, err := template.New("note").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
