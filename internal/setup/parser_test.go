package setup

import (
	"strings"
	"testing"
)

func TestParseRoutineBlocks_BasicSpanish(t *testing.T) {
	input := "Me levanto a las 7, desayuno, y arranco a trabajar de 9 a 18. Almuerzo tipo 1 de la tarde. Voy al gym a las 18:30, ceno a las 21 y me duermo tipo 23."
	blocks := ParseRoutineBlocks(input)

	if len(blocks) < 5 {
		t.Errorf("expected at least 5 blocks, got %d: %+v", len(blocks), blocks)
	}

	expected := map[string]struct {
		category string
		start    string
		minDur   int
	}{
		"wake_up":   {start: "07:00", minDur: 10},
		"work":      {start: "09:00", minDur: 300},
		"lunch":     {start: "13:00", minDur: 10},
		"exercise":  {start: "18:30", minDur: 10},
		"dinner":    {start: "21:00", minDur: 10},
		"sleep":     {start: "23:00", minDur: 10},
	}

	for _, b := range blocks {
		exp, ok := expected[b.Category]
		if !ok {
			continue
		}
		if b.Start != exp.start {
			t.Errorf("%s: start = %q, want %q", b.Category, b.Start, exp.start)
		}
		if b.DurationMinutes < exp.minDur {
			t.Errorf("%s: duration %d < %d", b.Category, b.DurationMinutes, exp.minDur)
		}
	}
}

func TestParseRoutineBlocks_AmPm(t *testing.T) {
	input := "I wake up at 7am, start work at 8:30 am. Lunch at 12pm. Gym at 6pm. Sleep at 10pm."
	blocks := ParseRoutineBlocks(input)

	categories := map[string]bool{}
	for _, b := range blocks {
		categories[b.Category] = true
	}

	checks := []string{"wake_up", "work", "lunch", "exercise", "sleep"}
	for _, c := range checks {
		if !categories[c] {
			t.Errorf("expected category %s, not found in %v", c, mapKeys(categories))
		}
	}

	for _, b := range blocks {
		switch b.Category {
		case "wake_up":
			if b.Start != "07:00" {
				t.Errorf("wake_up start = %q, want 07:00", b.Start)
			}
		case "lunch":
			if b.Start != "12:00" {
				t.Errorf("lunch start = %q, want 12:00", b.Start)
			}
		case "sleep":
			if b.Start != "22:00" {
				t.Errorf("sleep start = %q, want 22:00", b.Start)
			}
		case "exercise":
			if b.Start != "18:00" {
				t.Errorf("exercise start = %q, want 18:00", b.Start)
			}
		}
	}
}

func TestParseRoutineBlocks_WeekdaysSpecified(t *testing.T) {
	input := "Trabajo de lunes a viernes de 9 a 18. Almuerzo a las 13."
	blocks := ParseRoutineBlocks(input)

	for _, b := range blocks {
		if !b.DaysSpecified {
			t.Logf("block %s: days %v, DaysSpecified=%v", b.Title, b.Days, b.DaysSpecified)
		}
	}

	for _, b := range blocks {
		// Both should pick up the "lunes a viernes" clause.
		weekday := strings.Join(b.Days, ",")
		expected := strings.Join(weekdayDays, ",")
		if weekday != expected {
			t.Errorf("%s: days = %v, want %v", b.Title, b.Days, weekdayDays)
		}
	}
}

func TestParseRoutineBlocks_AllDays(t *testing.T) {
	input := "Me despierto todos los días a las 8. Ceno a las 21."
	blocks := ParseRoutineBlocks(input)

	for _, b := range blocks {
		if len(b.Days) != 7 {
			t.Errorf("%s: expected 7 days, got %d: %v", b.Title, len(b.Days), b.Days)
		}
	}
}

func TestParseRoutineBlocks_DurationFromRange(t *testing.T) {
	input := "Trabajo de 9 a 17."
	blocks := ParseRoutineBlocks(input)

	var work *InferredBlock
	for i := range blocks {
		if blocks[i].Category == "work" {
			work = &blocks[i]
			break
		}
	}
	if work == nil {
		t.Fatal("work block not found")
	}
	// 9:00 to 17:00 = 8h = 480m
	if work.DurationMinutes != 480 {
		t.Errorf("work duration = %d, want 480", work.DurationMinutes)
	}
}

func TestParseRoutineBlocks_Study(t *testing.T) {
	input := "Estudio en la facultad de 8 a 12. Almuerzo a las 12:30."
	blocks := ParseRoutineBlocks(input)

	found := false
	for _, b := range blocks {
		if b.Category == "study" {
			found = true
			if b.DurationMinutes != 240 {
				t.Errorf("study duration = %d, want 240", b.DurationMinutes)
			}
		}
	}
	if !found {
		t.Error("study block not found")
	}
}

func TestParseRoutineBlocks_Empty(t *testing.T) {
	blocks := ParseRoutineBlocks("no hay nada de horarios acá")
	if len(blocks) != 0 {
		t.Errorf("expected 0 blocks, got %d", len(blocks))
	}
}

func TestParseRoutineBlocks_SortedByStart(t *testing.T) {
	input := "Ceno a las 22. Me levanto a las 6. Almuerzo a las 13. Trabajo de 9 a 17."
	blocks := ParseRoutineBlocks(input)

	for i := 1; i < len(blocks); i++ {
		if blocks[i].Start < blocks[i-1].Start {
			t.Errorf("blocks not sorted: %s (%s) before %s (%s)",
				blocks[i-1].Title, blocks[i-1].Start, blocks[i].Title, blocks[i].Start)
		}
	}
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
