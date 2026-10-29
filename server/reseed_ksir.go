package server

import (
	"log"

	"github.com/Autumn-27/artex/agent"
)

// reseedKSIRWorkerPrompt inserts the KSIR offensive worker prompt as a new version
// and switches current_prompt_id to it. Idempotent: guarded by flag
// "ksir_weaponized_prompts_v1". Fresh DBs need nothing — seedPrompts already
// seeds the latest default (now the KSIR template) directly.
func (s *Server) reseedKSIRWorkerPrompt() {
	const flag = "ksir_weaponized_prompts_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	defer func() { _ = s.m.pg.SetSetting(flag, "true") }() // attempt only once regardless of outcome
	a, err := s.m.pg.GetAgentByKey("worker")
	if err != nil || a == nil {
		return // fresh DB without agent rows yet: seedPrompts seeds the KSIR default directly
	}
	tmpl := agent.BuiltinPromptSeeds()["worker"]
	if tmpl == "" {
		return
	}
	// Fresh DB: seedPrompts already seeded the latest default → current version equals code default, no duplicate needed.
	if cur, err := s.m.pg.CurrentPrompt(a.ID); err == nil && cur == tmpl {
		return
	}
	if _, err := s.m.pg.ResetPromptToDefault(a.ID, tmpl); err != nil {
		log.Printf("[KSIR] failed to reseed worker prompt: %v", err)
		return
	}
	log.Printf("[KSIR] worker prompt reseeded to KSIR offensive executor doctrine (ksir_weaponized_prompts_v1)")
}

// reseedKSIRReporterPrompt inserts the KSIR exfil curation prompt as a new version
// for the reporter agent. Idempotent: guarded by flag "ksir_reporter_curation_v1".
func (s *Server) reseedKSIRReporterPrompt() {
	const flag = "ksir_reporter_curation_v1"
	if v, _, _ := s.m.pg.GetSetting(flag); v == "true" {
		return
	}
	defer func() { _ = s.m.pg.SetSetting(flag, "true") }() // attempt only once regardless of outcome
	a, err := s.m.pg.GetAgentByKey("reporter")
	if err != nil || a == nil {
		return // reporter not seeded yet — seedReporterAgent will seed KSIR prompt directly
	}
	tmpl := agent.ReporterDefaultPrompt
	if tmpl == "" {
		return
	}
	// Fresh DB: SeedPromptIfEmpty already used the KSIR constant → current version matches, no duplicate needed.
	if cur, err := s.m.pg.CurrentPrompt(a.ID); err == nil && cur == tmpl {
		return
	}
	if _, err := s.m.pg.ResetPromptToDefault(a.ID, tmpl); err != nil {
		log.Printf("[KSIR] failed to reseed reporter prompt: %v", err)
		return
	}
	log.Printf("[KSIR] reporter prompt reseeded to KSIR exfil curation doctrine (ksir_reporter_curation_v1)")
}
