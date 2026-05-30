package setup

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/schedule"
)

// RunInterview executes the adaptive routine onboarding flow.
func RunInterview(cfg *config.Config, store *schedule.Store) error {
	scanner := bufio.NewScanner(os.Stdin)

	// Step 1: Check for existing events.
	events, err := store.ListEvents()
	if err != nil {
		return fmt.Errorf("schedule: list events: %w", err)
	}
	if len(events) > 0 {
		fmt.Print("Ya tenés rutinas guardadas. ¿Querés rehacer la entrevista? (s/N): ")
		if !scanLower(scanner) || !affirmative(scanner.Text()) {
			fmt.Println("Cancelado. Tu rutina actual no se modificó.")
			return nil
		}
	}

	// Step 2: Open-ended question.
	fmt.Println()
	fmt.Println("Contame cómo es tu día típico. ¿A qué hora te levantás, trabajás/estudiás, almorzás, hacés ejercicio, etc.?")
	fmt.Println("No hace falta que sea exacto, contame en tus palabras.")
	fmt.Print("> ")
	if !scanner.Scan() {
		return fmt.Errorf("no se recibió respuesta")
	}
	raw := strings.TrimSpace(scanner.Text())
	if raw == "" {
		return fmt.Errorf("no se recibió respuesta")
	}

	// Parse blocks.
	blocks := ParseRoutineBlocks(raw)
	if len(blocks) == 0 {
		fmt.Println("No pude detectar bloques de rutina en tu descripción.")
		fmt.Println("Probá usando horarios como 'a las 7', 'de 9 a 18', o 'a las 8am'.")
		return nil
	}

	// Step 3: Confirm ambiguous blocks — only those where days weren't specified.
	for i := range blocks {
		if blocks[i].DaysSpecified {
			continue
		}
		fmt.Println()
		label := strings.ToLower(blocks[i].Title)
		fmt.Printf("Entiendo que %s a las %s. ¿Es de lunes a viernes o todos los días?\n> ", label, blocks[i].Start)
		if !scanner.Scan() {
			break
		}
		resp := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if isWeekdayResponse(resp) {
			blocks[i].Days = weekdayDays
		} else {
			blocks[i].Days = allDays
		}
		blocks[i].Confirmed = true
	}

	// Step 4: Productive hours.
	fmt.Println()
	fmt.Println("¿En qué horarios preferís enfocarte en tareas importantes? (ej: mañana, tarde, noche, de 9 a 13)")
	fmt.Print("> ")
	if !scanner.Scan() {
		fmt.Println("Interrumpido. Guardando lo que se alcanzó a inferir...")
	} else {
		productiveHours := parseProductiveHours(scanner.Text())
		if len(productiveHours) > 0 {
			cfg.ProductiveHours = productiveHours
		} else {
			cfg.ProductiveHours = config.DefaultProductiveHours()
		}
	}

	// Step 5: Preview.
	fmt.Println()
	fmt.Println("Así quedaría tu rutina semanal:")
	fmt.Println()
	fmt.Println(renderPreviewTable(blocks))
	fmt.Println()

	fmt.Print("¿Confirmás? (s/N): ")
	if !scanLower(scanner) || !affirmative(scanner.Text()) {
		fmt.Println("Cancelado. No se guardaron cambios.")
		return nil
	}

	// Step 6: Save events.
	for _, b := range blocks {
		ev := schedule.ScheduleEvent{
			Title:           b.Title,
			Type:            schedule.EventTypeRoutine,
			Start:           b.Start,
			DurationMinutes: b.DurationMinutes,
			Days:            b.Days,
			Category:        b.Category,
		}
		if _, err := store.AddEvent(ev); err != nil {
			return fmt.Errorf("schedule: add event %q: %w", b.Title, err)
		}
	}

	// Save productive hours to config.
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("config: save: %w", err)
	}

	fmt.Println()
	fmt.Println("¡Listo! Tu rutina quedó guardada. Podés verla con `silo server` y usar `preview_schedule`.")
	return nil
}

func affirmative(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return false
	}
	return s == "s" || s == "si" || s == "sí" || s == "yes" || s == "y" || s == "dale" || s == "bueno" || s == "ok"
}

func isWeekdayResponse(s string) bool {
	re := regexp.MustCompile(`lunes a viernes|de lunes a viernes|entre semana|weekday|l-v|mon-fri|s[oó]lo semana|d[ií]as?\s+h[aá]biles|d[ií]as?\s+de\s+semana`)
	return re.MatchString(s)
}

// parseProductiveHours converts natural language to [][2]string intervals.
func parseProductiveHours(s string) [][2]string {
	s = strings.ToLower(strings.TrimSpace(s))

	// Try structured: "de X a Y, de X a Y"
	rangeRe := regexp.MustCompile(`de\s+(\d{1,2})(?::(\d{2}))?\s+(?:a|—|-)\s+(?:las?\s+)?(\d{1,2})(?::(\d{2}))?`)
	matches := rangeRe.FindAllStringSubmatch(s, -1)
	if len(matches) > 0 {
		var out [][2]string
		for _, m := range matches {
			h, _ := strconv.Atoi(m[1])
			min := "00"
			if m[2] != "" {
				min = twoDigitS(minStr(m[2]))
			}
			eh, _ := strconv.Atoi(m[3])
			emin := "00"
			if m[4] != "" {
				emin = twoDigitS(minStr(m[4]))
			}
			out = append(out, [2]string{
				fmt.Sprintf("%02d:%s", h, min),
				fmt.Sprintf("%02d:%s", eh, emin),
			})
		}
		return out
	}

	// Named periods.
	var out [][2]string
	if strings.Contains(s, "mañana") || strings.Contains(s, "morning") {
		out = append(out, [2]string{"08:00", "12:00"})
	}
	if strings.Contains(s, "tarde") || strings.Contains(s, "afternoon") {
		out = append(out, [2]string{"14:00", "18:00"})
	}
	if strings.Contains(s, "noche") || strings.Contains(s, "evening") || strings.Contains(s, "night") {
		out = append(out, [2]string{"20:00", "23:00"})
	}
	if len(out) > 0 {
		return out
	}

	// "todo el día" or "all day"
	if strings.Contains(s, "todo") || strings.Contains(s, "all day") {
		return [][2]string{{"08:00", "18:00"}}
	}

	return config.DefaultProductiveHours()
}

func twoDigitS(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return "00"
	}
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func minStr(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// renderPreviewTable generates a markdown table of the inferred routine.
func renderPreviewTable(blocks []InferredBlock) string {
	sorted := make([]InferredBlock, len(blocks))
	copy(sorted, blocks)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })

	var b strings.Builder
	b.WriteString("| Hora | Actividad | Duración | Días |\n")
	b.WriteString("|------|-----------|----------|------|\n")
	for _, bl := range sorted {
		b.WriteString("| ")
		b.WriteString(bl.Start)
		b.WriteString(" | ")
		b.WriteString(bl.Title)
		b.WriteString(" | ")
		b.WriteString(fmtDuration(bl.DurationMinutes))
		b.WriteString(" | ")
		b.WriteString(fmtDays(bl.Days))
		b.WriteString(" |\n")
	}
	return b.String()
}

func fmtDuration(m int) string {
	if m < 60 {
		return fmt.Sprintf("%dm", m)
	}
	h := m / 60
	r := m % 60
	if r == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, r)
}

func fmtDays(days []string) string {
	if len(days) == 7 {
		return "todos los días"
	}
	if len(days) == 5 {
		isWeekday := true
		for _, d := range days {
			if d == "sat" || d == "sun" {
				isWeekday = false
				break
			}
		}
		if isWeekday {
			return "lunes a viernes"
		}
	}
	return strings.Join(days, ", ")
}

func scanLower(scanner *bufio.Scanner) bool {
	if !scanner.Scan() {
		return false
	}
	return true
}
