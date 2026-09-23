package domain

import "time"

type Scenario struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Description          string         `json:"description"`
	Boundaries           string         `json:"boundaries"`
	Examples             []string       `json:"examples"`
	RequiredSlots        []string       `json:"required_slots"`
	ResponseRU           string         `json:"response_ru"`
	ResponseKK           string         `json:"response_kk"`
	FactIDs              []string       `json:"fact_ids"`
	RequiresConfirmation bool           `json:"requires_confirmation"`
	Raw                  map[string]any `json:"-"`
}
type Alternative struct {
	ScenarioID string `json:"scenario_id"`
	Reason     string `json:"reason"`
}
type Slot struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Decision struct {
	ScenarioID   string        `json:"scenario_id"`
	Status       string        `json:"status"`
	Confidence   float64       `json:"confidence"`
	Language     string        `json:"language"`
	Reason       string        `json:"reason"`
	Alternatives []Alternative `json:"alternatives"`
	Slots        []Slot        `json:"slots"`
	Pending      []string      `json:"pending"`
}
type Call struct {
	Attempt          int     `json:"attempt"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	LatencyMS        float64 `json:"latency_ms"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CachedTokens     int     `json:"cached_tokens"`
	Error            string  `json:"error,omitempty"`
}
type Timing struct {
	RoutingMS      float64  `json:"routing_ms"`
	PolicyMS       float64  `json:"policy_ms"`
	ServerMS       float64  `json:"server_ms"`
	STTMS          *float64 `json:"stt_ms,omitempty"`
	TTSFirstByteMS *float64 `json:"tts_first_byte_ms,omitempty"`
	EndToAudioMS   *float64 `json:"end_to_audio_ms,omitempty"`
	InputKind      string   `json:"input_kind"`
}
type Turn struct {
	ID               string    `json:"id"`
	Text             string    `json:"text"`
	Reply            string    `json:"reply"`
	Decision         Decision  `json:"decision"`
	ScenarioName     string    `json:"scenario_name"`
	PreviousScenario string    `json:"previous_scenario"`
	TopicChanged     bool      `json:"topic_changed"`
	Source           string    `json:"source"`
	Calls            []Call    `json:"calls"`
	Timing           Timing    `json:"timing"`
	Warnings         []string  `json:"warnings"`
	CreatedAt        time.Time `json:"created_at"`
}
type Session struct {
	ID        string    `json:"id"`
	Turns     []Turn    `json:"turns"`
	Active    string    `json:"active"`
	Pending   []string  `json:"pending"`
	CreatedAt time.Time `json:"created_at"`
}
type RouteInput struct {
	Text    string   `json:"text"`
	History []Turn   `json:"history"`
	Active  string   `json:"active"`
	Pending []string `json:"pending"`
}
