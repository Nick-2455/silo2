package identity

// These prompt templates are placeholders for future enrichment via an LLM.
// MVP scaffolding keeps them as exported constants with no runtime dependency.

const CVPrompt = `You are helping generate a concise CV from an Identity profile.
Use the provided skills, projects, and evidence. Keep it factual and avoid embellishment.
Return Markdown.`

const LinkedInPrompt = `You are helping generate a LinkedIn "About" section from an Identity profile.
Tone: confident, practical, technically credible. Avoid buzzwords.
Return plain text.`

const ProfessionalBioPrompt = `You are helping generate a professional bio from an Identity profile.
Keep it short, specific, and backed by evidence when possible.
Return Markdown.`
