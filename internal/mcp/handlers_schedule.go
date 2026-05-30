package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/nicolasperalta/silo2/internal/config"
	"github.com/nicolasperalta/silo2/internal/schedule"
)

func addScheduleEventTool() mcp.Tool {
	return mcp.NewTool("add_schedule_event",
		mcp.WithDescription("Add a fixed or routine schedule event to Silo's local schedule JSON."),
		mcp.WithString("title", mcp.Required(), mcp.Description("Event title, e.g. Work, Class, Gym")),
		mcp.WithString("start", mcp.Required(), mcp.Description("Start time in HH:MM format")),
		mcp.WithNumber("duration_minutes", mcp.Required(), mcp.Description("Duration in minutes")),
		mcp.WithArray("days", mcp.Description("Weekday keys (mon..sun) or explicit YYYY-MM-DD dates"), mcp.WithStringItems()),
		mcp.WithString("type", mcp.Description("Event type: fixed or routine"), mcp.Enum("fixed", "routine")),
		mcp.WithString("category", mcp.Description("Optional category, e.g. work, health, study")),
	)
}

func listScheduleEventsTool() mcp.Tool {
	return mcp.NewTool("list_schedule_events",
		mcp.WithDescription("List schedule events resolved for a date. Returns JSON for the agent to render."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format. Defaults to today.")),
	)
}

func removeScheduleEventTool() mcp.Tool {
	return mcp.NewTool("remove_schedule_event",
		mcp.WithDescription("Remove a schedule event by ID."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Schedule event ID")),
	)
}

func getFreeSlotsTool() mcp.Tool {
	return mcp.NewTool("get_free_slots",
		mcp.WithDescription("Return free time slots for a date, based on the local schedule."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format. Defaults to today.")),
		mcp.WithString("start", mcp.Description("Search window start HH:MM. Defaults to 06:00.")),
		mcp.WithString("end", mcp.Description("Search window end HH:MM. Defaults to 23:00.")),
	)
}

func handleAddScheduleEvent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	title, err := req.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	start, err := req.RequireString("start")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	duration, err := numberArg(req, "duration_minutes")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	evType := schedule.EventType(req.GetString("type", string(schedule.EventTypeFixed)))
	if evType != schedule.EventTypeFixed && evType != schedule.EventTypeRoutine {
		return mcp.NewToolResultError("type must be fixed or routine"), nil
	}

	ev := schedule.ScheduleEvent{
		Title:           title,
		Type:            evType,
		Start:           start,
		DurationMinutes: int(duration),
		Days:            stringSliceArg(req, "days"),
		Category:        req.GetString("category", ""),
	}

	if err := schedule.ValidateEvent(ev); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	added, err := scheduleStore().AddEvent(ev)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to add schedule event: %v", err)), nil
	}
	return jsonResult(map[string]any{"event": added})
}

func handleListScheduleEvents(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	date := req.GetString("date", time.Now().Format("2006-01-02"))
	sch, err := scheduleStore().Load()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load schedule: %v", err)), nil
	}
	events, err := schedule.ResolveDay(sch, date)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"date": date, "events": events, "count": len(events)})
}

func handleRemoveScheduleEvent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if err := scheduleStore().RemoveEvent(id); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"removed": true, "id": id})
}

func handleGetFreeSlots(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	date := req.GetString("date", time.Now().Format("2006-01-02"))
	start := req.GetString("start", "06:00")
	end := req.GetString("end", "23:00")
	sch, err := scheduleStore().Load()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load schedule: %v", err)), nil
	}
	slots, err := schedule.FreeSlots(sch, date, start, end)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(map[string]any{"date": date, "start": start, "end": end, "slots": slots, "count": len(slots)})
}

func scheduleStore() *schedule.Store {
	path := deps.Config.SchedulePath
	if path == "" {
		path = config.DefaultSchedulePath()
	}
	return schedule.NewStore(path)
}

func previewScheduleTool() mcp.Tool {
	return mcp.NewTool("preview_schedule",
		mcp.WithDescription("Render a Markdown table preview of the schedule for a date, including events and free slots."),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format. Defaults to today.")),
		mcp.WithString("start", mcp.Description("Search window start HH:MM. Defaults to 06:00.")),
		mcp.WithString("end", mcp.Description("Search window end HH:MM. Defaults to 23:00.")),
	)
}

func handlePreviewSchedule(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	date := req.GetString("date", time.Now().Format("2006-01-02"))
	start := req.GetString("start", "06:00")
	end := req.GetString("end", "23:00")

	sch, err := scheduleStore().Load()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to load schedule: %v", err)), nil
	}

	events, err := schedule.ResolveDay(sch, date)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	slots, err := schedule.FreeSlots(sch, date, start, end)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	md := renderSchedulePreview(date, events, slots, start, end)
	return jsonResult(map[string]any{
		"date":     date,
		"markdown": md,
		"events":   len(events),
		"slots":    len(slots),
	})
}

func renderSchedulePreview(date string, events []schedule.ResolvedEvent, slots []schedule.TimeSlot, windowStart, windowEnd string) string {
	var b strings.Builder
	b.WriteString("### Schedule for ")
	b.WriteString(date)
	b.WriteString("\n\n")

	if len(events) == 0 {
		b.WriteString("*No events scheduled.*\n\n")
	} else {
		b.WriteString("| Start | End | Event | Type | Duration |\n")
		b.WriteString("|-------|-----|-------|------|----------|\n")
		for _, ev := range events {
			b.WriteString("| ")
			b.WriteString(ev.Start)
			b.WriteString(" | ")
			b.WriteString(ev.End)
			b.WriteString(" | ")
			b.WriteString(ev.Title)
			b.WriteString(" | ")
			b.WriteString(string(ev.Type))
			b.WriteString(" | ")
			b.WriteString(fmtMinutes(ev.DurationMinutes))
			b.WriteString(" |\n")
		}
		b.WriteString("\n")
	}

	if len(slots) == 0 {
		b.WriteString("**Free slots (")
		b.WriteString(windowStart)
		b.WriteString("–")
		b.WriteString(windowEnd)
		b.WriteString("):** *None.*\n")
	} else {
		b.WriteString("**Free slots (")
		b.WriteString(windowStart)
		b.WriteString("–")
		b.WriteString(windowEnd)
		b.WriteString("):**\n")
		for _, s := range slots {
			b.WriteString("- ")
			b.WriteString(s.Start)
			b.WriteString(" – ")
			b.WriteString(s.End)
			b.WriteString(" (")
			b.WriteString(fmtMinutes(s.DurationMinutes))
			b.WriteString(")\n")
		}
	}
	return b.String()
}

func fmtMinutes(m int) string {
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
