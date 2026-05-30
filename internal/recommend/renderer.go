package recommend

import (
	"fmt"
	"strings"
)

// RenderMarkdown formats recommendations as a human-readable markdown block.
func RenderMarkdown(date string, freeMinutes int, recs []Recommendation) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Recomendaciones para %s (%d minutos libres)\n\n", date, freeMinutes))

	groups := groupByLabel(recs)
	order := []string{"watch-now", "watch-later", "expand", "requires-prerequisite", "skip"}

	for _, label := range order {
		items, ok := groups[label]
		if !ok || len(items) == 0 {
			continue
		}

		b.WriteString(fmt.Sprintf("### %s\n\n", labelHeader(label)))

		for _, r := range items {
			b.WriteString(fmt.Sprintf("- **%s**", r.Title))
			if r.Source != "" {
				b.WriteString(fmt.Sprintf(" — `%s`", r.Source))
			}
			if r.DurationEstimate > 0 {
				b.WriteString(fmt.Sprintf(" — %d min", r.DurationEstimate))
			}
			if r.Category != "" {
				b.WriteString(fmt.Sprintf(" — %s", r.Category))
			}
			b.WriteString(fmt.Sprintf(" (%s)", r.Reason))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}

func groupByLabel(recs []Recommendation) map[string][]Recommendation {
	groups := make(map[string][]Recommendation)
	for _, r := range recs {
		groups[r.Label] = append(groups[r.Label], r)
	}
	return groups
}

func labelHeader(label string) string {
	switch label {
	case "watch-now":
		return "Ver ahora"
	case "watch-later":
		return "Ver después"
	case "expand":
		return "Expandir"
	case "requires-prerequisite":
		return "Requiere prerrequisito"
	case "skip":
		return "Saltar"
	default:
		return label
	}
}
