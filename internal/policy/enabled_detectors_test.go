package policy

import "testing"

func TestEnabledDetectorSet_Empty(t *testing.T) {
	c := &Config{}
	if got := c.EnabledDetectorSet(); got != nil {
		t.Errorf("EnabledDetectorSet on empty config = %v, want nil", got)
	}
}

func TestEnabledDetectorSet_Populated(t *testing.T) {
	c := &Config{Rules: Rules{
		EnabledDetectors: []string{
			"aiPromptInjectionRisk",
			"  aiHardcodedAPIKey  ",
			"",
		},
	}}
	got := c.EnabledDetectorSet()
	if got == nil {
		t.Fatal("EnabledDetectorSet returned nil for non-empty config")
	}
	if !got["aiPromptInjectionRisk"] {
		t.Errorf("aiPromptInjectionRisk not enabled; got %v", got)
	}
	if !got["aiHardcodedAPIKey"] {
		t.Errorf("aiHardcodedAPIKey not enabled (whitespace not trimmed); got %v", got)
	}
	// Empty strings should be dropped, not become an empty-string key.
	if got[""] {
		t.Errorf("empty-string opt-in should be filtered; got %v", got)
	}
}

func TestEnabledDetectorSet_Nil(t *testing.T) {
	var c *Config
	if got := c.EnabledDetectorSet(); got != nil {
		t.Errorf("EnabledDetectorSet on nil config = %v, want nil", got)
	}
}

func TestConfig_NotEmpty_WithEnabledDetectors(t *testing.T) {
	c := &Config{Rules: Rules{EnabledDetectors: []string{"foo"}}}
	if c.IsEmpty() {
		t.Error("IsEmpty should be false when EnabledDetectors is set")
	}
}
