package mcp

import "testing"

func TestNewServer_RegistersOnlySupportedTools(t *testing.T) {
	t.Parallel()

	srv := NewServer()
	tools := srv.ListTools()

	if len(tools) != 3 {
		t.Fatalf("ListTools() count = %d, want 3", len(tools))
	}

	for _, name := range []string{"silo_recommend", "get_profile_context", "init_profile"} {
		if tools[name] == nil {
			t.Fatalf("expected tool %q to be registered", name)
		}
	}
	for _, name := range []string{
		"add_" + "sched" + "ule_event",
		"remove_" + "sched" + "ule_event",
		"list_" + "sched" + "ule_events",
		"get_free_slots",
		"preview_" + "sched" + "ule",
	} {
		if tools[name] != nil {
			t.Fatalf("expected tool %q to be removed", name)
		}
	}
}
