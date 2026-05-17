package markdown

// Templates are intentionally timestamp-free so re-rendering with unchanged
// input produces byte-identical output (idempotent generation).

const identityTemplate = `---
type: identity
generated_by: silo
---

# Identity

Name: {{ .Identity.Name }}
Role: {{ .Identity.Role }}

## Areas
{{- if .Identity.Areas }}
{{- range .Identity.Areas }}
- {{ . }}
{{- end }}
{{- else }}
- (none)
{{- end }}

## Goals
{{- if .Identity.Goals }}
{{- range .Identity.Goals }}
- {{ . }}
{{- end }}
{{- else }}
- (none)
{{- end }}

## Evidence
{{- if .Identity.Evidence }}
{{- range .Identity.Evidence }}
- {{ .Source }}: {{ .Summary }}
{{- end }}
{{- else }}
- (none)
{{- end }}

## Related Notes
- [[Skills]]
- [[Projects]]
- [[Outputs]]
`

const skillsTemplate = `---
type: skills
generated_by: silo
---

# Skills
{{- if .Identity.Skills }}
{{- range .Identity.Skills }}
- {{ . }}
{{- end }}
{{- else }}
- (none)
{{- end }}

## Interests
{{- if .Identity.Interests }}
{{- range .Identity.Interests }}
- {{ . }}
{{- end }}
{{- else }}
- (none)
{{- end }}
`

const projectsTemplate = `---
type: projects
generated_by: silo
---

# Projects
{{- if .Identity.Projects }}
{{ range .Identity.Projects }}
## {{ .Name }}

Status: {{ .Status }}

{{ .Description }}
{{ end -}}
{{- else }}

(none)
{{- end }}
`

const outputsTemplate = `---
type: outputs
generated_by: silo
---

# Outputs

- IdentityProfile: {{ .Identity.Outputs.IdentityProfile }}
- CV: {{ .Identity.Outputs.CV }}
- LinkedIn: {{ .Identity.Outputs.LinkedIn }}
- Portfolio: {{ .Identity.Outputs.Portfolio }}
- ProfessionalBio: {{ .Identity.Outputs.ProfessionalBio }}
`
