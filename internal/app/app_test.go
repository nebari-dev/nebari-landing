// Copyright 2026, OpenTeams.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"testing"
)

func TestThemeIcon_AsIconValue(t *testing.T) {
	tests := []struct {
		name string
		in   ThemeIcon
		want IconValue
	}{
		{"plain", ThemeIcon{Plain: "https://example.com/icon.svg"}, IconValue{Plain: "https://example.com/icon.svg"}},
		{"light and dark", ThemeIcon{Light: "https://example.com/light.svg", Dark: "https://example.com/dark.svg"}, IconValue{Light: "https://example.com/light.svg", Dark: "https://example.com/dark.svg"}},
		{"all fields", ThemeIcon{Plain: "p", Light: "l", Dark: "d"}, IconValue{Plain: "p", Light: "l", Dark: "d"}},
		{"zero value", ThemeIcon{}, IconValue{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.AsIconValue(); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIconValue_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		v    IconValue
		want string
	}{
		{"plain string", IconValue{Plain: "https://example.com/icon.svg"}, `"https://example.com/icon.svg"`},
		{"empty plain", IconValue{}, `""`},
		{"light and dark", IconValue{Light: "https://example.com/light.svg", Dark: "https://example.com/dark.svg"}, `{"light":"https://example.com/light.svg","dark":"https://example.com/dark.svg"}`},
		{"light only", IconValue{Light: "https://example.com/light.svg"}, `{"light":"https://example.com/light.svg"}`},
		{"dark only", IconValue{Dark: "https://example.com/dark.svg"}, `{"dark":"https://example.com/dark.svg"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatalf("MarshalJSON error: %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestIconValue_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  IconValue
	}{
		{"plain string", `"https://example.com/icon.svg"`, IconValue{Plain: "https://example.com/icon.svg"}},
		{"empty string", `""`, IconValue{Plain: ""}},
		{"light and dark", `{"light":"https://example.com/light.svg","dark":"https://example.com/dark.svg"}`, IconValue{Light: "https://example.com/light.svg", Dark: "https://example.com/dark.svg"}},
		{"light only", `{"light":"https://example.com/light.svg"}`, IconValue{Light: "https://example.com/light.svg"}},
		{"dark only", `{"dark":"https://example.com/dark.svg"}`, IconValue{Dark: "https://example.com/dark.svg"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got IconValue
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIconValue_String(t *testing.T) {
	tests := []struct {
		name string
		v    IconValue
		want string
	}{
		{"plain only", IconValue{Plain: "https://example.com/icon.svg"}, "https://example.com/icon.svg"},
		{"light and dark returns light", IconValue{Light: "https://example.com/light.svg", Dark: "https://example.com/dark.svg"}, "https://example.com/light.svg"},
		{"dark only", IconValue{Dark: "https://example.com/dark.svg"}, "https://example.com/dark.svg"},
		{"empty", IconValue{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIconValue_RoundTrip(t *testing.T) {
	originals := []IconValue{
		{Plain: "https://example.com/icon.svg"},
		{Light: "https://example.com/light.svg", Dark: "https://example.com/dark.svg"},
		{Light: "https://example.com/light.svg"},
		{Dark: "https://example.com/dark.svg"},
	}
	for _, orig := range originals {
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal %+v: %v", orig, err)
		}
		var got IconValue
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", data, err)
		}
		if got != orig {
			t.Errorf("round-trip failed: got %+v, want %+v", got, orig)
		}
	}
}
