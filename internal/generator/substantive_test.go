package generator

import "testing"

func TestIsSubstantive(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "empty string",
			text: "",
			want: false,
		},
		{
			name: "only headings",
			text: "# API\n## Endpoints\n### GET /users\n",
			want: false,
		},
		{
			name: "no endpoints found",
			text: "# API\n\nNo API endpoints were found.\n",
			want: false,
		},
		{
			name: "no endpoints found mixed case",
			text: "# API\n\nNo Api Endpoints Were Found in the codebase.\n",
			want: false,
		},
		{
			name: "mostly UNKNOWN",
			text: "# Runbook\n\n## Deployment\nStatus: UNKNOWN\nRollback: UNKNOWN\nMonitoring: UNKNOWN\nAlerts: UNKNOWN\n",
			want: false,
		},
		{
			name: "substantive architecture",
			text: "# Architecture\n\nThe system uses a layered architecture with Scanner, FactModel, LLM, Generator, and Renderer layers.\n\n## Components\n\n- Scanner: AST-based repository analysis\n- Generator: Template rendering with LLM pipeline\n",
			want: true,
		},
		{
			name: "substantive api",
			text: "# API\n\n## POST /api/v1/users\n\nCreates a new user account.\n\n### Request Body\n\n```json\n{\"name\": \"string\"}\n```\n",
			want: true,
		},
		{
			name: "some unknown but mostly real",
			text: "# Runbook\n\n## Deployment\nPlatform: Kubernetes\nCI: GitHub Actions\nRollback: UNKNOWN\n",
			want: true,
		},
		{
			name: "whitespace only",
			text: "   \n\n  \n  ",
			want: false,
		},
		{
			name: "insufficient data",
			text: "# Data Model\n\nInsufficient data to generate a data model.\n",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSubstantive(tt.text)
			if got != tt.want {
				t.Errorf("IsSubstantive() = %v, want %v", got, tt.want)
			}
		})
	}
}
