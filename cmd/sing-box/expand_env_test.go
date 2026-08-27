package main

import (
	"strings"
	"testing"
)

func TestExpandEnvVariables(t *testing.T) {
	t.Setenv("TEST_EXPAND_UUID", "abc-123")
	t.Setenv("TEST_EXPAND_PORT", "8080")
	content := []byte(`{"uuid": "${TEST_EXPAND_UUID}", "port": ${TEST_EXPAND_PORT}}`)
	expanded, err := expandEnvVariables(content, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"uuid": "abc-123", "port": 8080}`
	if string(expanded) != expected {
		t.Fatalf("expected %s, got %s", expected, string(expanded))
	}
}

func TestExpandEnvVariablesMissing(t *testing.T) {
	content := []byte(`{"uuid": "${TEST_MISSING_A}-${TEST_MISSING_B}", "x": "${TEST_MISSING_A}"}`)
	_, err := expandEnvVariables(content, "config.json")
	if err == nil {
		t.Fatal("expected error for missing variables")
	}
	if !strings.Contains(err.Error(), "TEST_MISSING_A") || !strings.Contains(err.Error(), "TEST_MISSING_B") {
		t.Fatalf("expected missing variable names in error, got: %v", err)
	}
}

func TestExpandEnvNoVariables(t *testing.T) {
	content := []byte(`{"log": {"level": "info"}, "note": "$HOME and $1 stay"}`)
	expanded, err := expandEnvVariables(content, "config.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(expanded) != string(content) {
		t.Fatalf("expected unchanged content, got %s", string(expanded))
	}
}
