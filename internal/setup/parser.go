package setup

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// activityDef maps keywords to block titles and default durations.
type activityDef struct {
	keywords        []string
	title           string
	category        string
	defaultDuration int
}

var activities = []activityDef{
	{keywords: []string{"levant", "despert", "wake"}, title: "Despertar", category: "wake_up", defaultDuration: 30},
	{keywords: []string{"trabaj", "work", "labur", "oficin", "offic"}, title: "Trabajo", category: "work", defaultDuration: 480},
	{keywords: []string{"estudi", "study", "facultad", "universidad", "clase", "class", "curso", "course"}, title: "Estudio", category: "study", defaultDuration: 180},
	{keywords: []string{"almuerz", "lunch", "almorzar", "comida", "comer"}, title: "Almuerzo", category: "lunch", defaultDuration: 60},
	{keywords: []string{"gym", "gimnasio", "ejercicio", "exercise", "correr", "run", "fitness", "deporte", "entren"}, title: "Ejercicio", category: "exercise", defaultDuration: 60},
	{keywords: []string{"cen", "cena", "cenar", "dinner"}, title: "Cena", category: "dinner", defaultDuration: 60},
	{keywords: []string{"dormir", "dorm", "duerm", "acost", "sleep", "bed", "cama"}, title: "Dormir", category: "sleep", defaultDuration: 480},
	{keywords: []string{"libre", "ocio", "hobby", "descans", "leer", "read", "guitar", "musica", "pelicul", "movie", "relaj"}, title: "Tiempo libre", category: "hobby", defaultDuration: 120},
}

var weekdayDays = []string{"mon", "tue", "wed", "thu", "fri"}
var allDays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

type timeRef struct {
	hour, minute       int
	endHour, endMinute int // 0 if single time, >0 if range
	pos                int
}

// ParseRoutineBlocks extracts routine blocks from free-text input.
func ParseRoutineBlocks(text string) []InferredBlock {
	text = strings.ToLower(strings.TrimSpace(text))
	defaultDays, daysSpecified := detectGlobalDays(text)
	timeRefs := extractTimeRefs(text)

	var blocks []InferredBlock
	used := make(map[int]bool)

	for _, tr := range timeRefs {
		if used[tr.pos] {
			continue
		}
		act := findClosestActivity(text, tr.pos)
		if act == nil {
			continue
		}

		h, m := normalize24h(tr.hour, tr.minute)
		start := formatHHMM(h, m)

		duration := act.defaultDuration
		if tr.endHour > 0 || tr.endMinute > 0 {
			eh, em := normalize24h(tr.endHour, tr.endMinute)
			// If end seems earlier than start (e.g. "de 9 a 5"), assume PM.
			if eh < h || (eh == h && em <= m) {
				eh += 12
			}
			dur := (eh*60 + em) - (h*60 + m)
			if dur > 0 {
				duration = dur
			}
		}

		// Resolve days for this block based on global or local detection.
		days := defaultDays
		if tr.pos > 0 {
			localStart := tr.pos - 150
			if localStart < 0 {
				localStart = 0
			}
			localEnd := tr.pos
			if localEnd > len(text) {
				localEnd = len(text)
			}
			localWindow := text[localStart:localEnd]
			localDays, localSpec := detectDays(localWindow)
			if localSpec {
				days = localDays
				daysSpecified = true
			}
		}

		block := InferredBlock{
			Title:           act.title,
			Start:           start,
			DurationMinutes: duration,
			Category:        act.category,
			Days:            days,
			DaysSpecified:   daysSpecified,
		}
		blocks = append(blocks, block)
		used[tr.pos] = true
	}

	// Sort blocks by start time.
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Start < blocks[j].Start
	})

	// Deduplicate by category.
	seen := map[string]bool{}
	var deduped []InferredBlock
	for _, b := range blocks {
		if !seen[b.Category] {
			seen[b.Category] = true
			deduped = append(deduped, b)
		}
	}

	return deduped
}

// detectGlobalDays scans the whole text for day-pattern clues.
func detectGlobalDays(text string) ([]string, bool) {
	d, s := detectDays(text)
	if s {
		return d, s
	}
	return allDays, false
}

// detectDays checks a text fragment for day specifications.
func detectDays(text string) ([]string, bool) {
	allRe := regexp.MustCompile(`todos los d[ií]as|cada d[ií]a|diario|every\s+day|daily|all\s+days`)
	if allRe.MatchString(text) {
		return allDays, true
	}

	wdRe := regexp.MustCompile(`lunes a viernes|de lunes a viernes|entre semana|weekday|l-v|mon-fri|d[ií]as? de semana|d[ií]as? h[aá]biles`)
	if wdRe.MatchString(text) {
		return weekdayDays, true
	}
	return allDays, false
}

// extractTimeRefs finds all time references in the text.
func extractTimeRefs(text string) []timeRef {
	var refs []timeRef

	// --- "a las X" or "a las X:YY" patterns.
	// Groups: 1=hour, 2=minutes(opt), 3=am|pm(opt).
	aLasRe := regexp.MustCompile(`a\s+(?:las?\s+)?(\d{1,2})(?::(\d{2}))?(?:\s*(am|pm))?`)
	for _, m := range aLasRe.FindAllStringSubmatchIndex(text, -1) {
		// Skip matches that are part of a "de X a Y" range (preceded by a digit).
		if m[0] > 2 {
			prev := text[m[0]-2 : m[0]]
			if prev[0] >= '0' && prev[0] <= '9' {
				continue
			}
		}
		h, _ := strconv.Atoi(submatch(text, m, 1))
		min := submatchInt(text, m, 2)
		ampm := submatch(text, m, 3)
		tr := timeRef{hour: h, minute: min, pos: m[0]}
		if ampm != "" {
			tr.hour = adjustAMPMHour(h, ampm)
		}
		refs = append(refs, tr)
	}

	// --- "de X a Y" range.
	// Groups: 1=startH, 2=startM(opt), 3=endH, 4=endM(opt).
	deARangeRe := regexp.MustCompile(`(?:de\s+)?(\d{1,2})(?::(\d{2}))?\s*(?:a|—|-)\s*(?:las?\s+)?(\d{1,2})(?::(\d{2}))?`)
	for _, m := range deARangeRe.FindAllStringSubmatchIndex(text, -1) {
		tr := timeRef{
			hour:      submatchInt(text, m, 1),
			minute:    submatchInt(text, m, 2),
			endHour:   submatchInt(text, m, 3),
			endMinute: submatchInt(text, m, 4),
			pos:       m[0],
		}
		refs = append(refs, tr)
	}

	// --- "Xam", "X:YY pm", etc.
	// Groups: 1=hour, 2=minutes(opt), 3=am|pm.
	ampmRe := regexp.MustCompile(`(\d{1,2})(?::(\d{2}))?\s*(am|pm)`)
	for _, m := range ampmRe.FindAllStringSubmatchIndex(text, -1) {
		h, _ := strconv.Atoi(submatch(text, m, 1))
		min := submatchInt(text, m, 2)
		ampm := submatch(text, m, 3)
		tr := timeRef{hour: h, minute: min, pos: m[0]}
		if ampm != "" {
			tr.hour = adjustAMPMHour(h, ampm)
		}
		refs = append(refs, tr)
	}

	// --- "tipo X" or "tipo las X".
	// Groups: 1=hour, 2=minutes(opt).
	tipoRe := regexp.MustCompile(`tipo\s+(?:las?\s+)?(\d{1,2})(?::(\d{2}))?`)
	for _, m := range tipoRe.FindAllStringSubmatchIndex(text, -1) {
		h, _ := strconv.Atoi(submatch(text, m, 1))
		min := submatchInt(text, m, 2)

		// Check for "de la tarde", "de la noche", "de la mañana" near the match.
		ctxStart := m[1] - 60
		if ctxStart < 0 {
			ctxStart = 0
		}
		ctxEnd := m[0] + 60
		if ctxEnd > len(text) {
			ctxEnd = len(text)
		}
		ctx := text[ctxStart:ctxEnd]
		if strings.Contains(ctx, "tarde") || strings.Contains(ctx, "noche") {
			h = adjustAMPMHour(h, "pm")
		}

		tr := timeRef{hour: h, minute: min, pos: m[0]}
		refs = append(refs, tr)
	}

	// Sort by position.
	sort.Slice(refs, func(i, j int) bool { return refs[i].pos < refs[j].pos })
	return refs
}

// submatch returns the matched text for capture group g, or "" if not matched.
// m is the result of FindAllStringSubmatchIndex.
// g is the capture group *index* in the regex (1-indexed, same as regex group).
func submatch(text string, m []int, group int) string {
	// group 'g' corresponds to submatch index g in the match pairs.
	// Each submatch is a [start, end] pair, so index in m is g*2.
	idx := group * 2
	if idx+1 >= len(m) || m[idx] < 0 {
		return ""
	}
	return text[m[idx]:m[idx+1]]
}

// submatchInt returns the integer value of a submatch, or 0 if not matched.
func submatchInt(text string, m []int, group int) int {
	s := submatch(text, m, group)
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func adjustAMPMHour(hour int, amPm string) int {
	switch strings.ToLower(amPm) {
	case "am":
		if hour == 12 {
			return 0
		}
		return hour
	case "pm":
		if hour == 12 {
			return 12
		}
		return hour + 12
	}
	return hour
}

// findClosestActivity finds the activity definition whose keywords appear
// nearest to the given position in text. Looks only at LEFT context (text
// before the time reference) because in natural language the activity
// usually precedes the time: "trabajo de 9 a 18" or "me levanto a las 7".
func findClosestActivity(text string, pos int) *activityDef {
	const window = 60
	start := pos - window
	if start < 0 {
		start = 0
	}
	// Only look at text BEFORE the time reference.
	snippet := text[start:pos]

	var best *activityDef
	bestDist := window + 1

	for i := range activities {
		for _, kw := range activities[i].keywords {
			idx := strings.LastIndex(snippet, kw)
			if idx < 0 {
				continue
			}
			// Distance from end of snippet (closest to the time).
			dist := len(snippet) - (idx + len(kw))
			if dist < bestDist {
				bestDist = dist
				best = &activities[i]
			}
		}
	}
	return best
}

func normalize24h(h, m int) (int, int) {
	if h > 23 {
		h = 23
	}
	if m > 59 {
		m = 59
	}
	return h, m
}

func formatHHMM(h, m int) string {
	if h < 10 {
		return "0" + strconv.Itoa(h) + ":" + twoDigit(m)
	}
	return strconv.Itoa(h) + ":" + twoDigit(m)
}

func twoDigit(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
