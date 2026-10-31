package common_test

import (
	"testing"

	cmm "c_trd/common"
)

// Sin t.Parallel: t.Setenv modifica el entorno del proceso
func TestCronjobsEnabledFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "unset keeps cronjobs on", value: "", want: true},
		{name: "true", value: "true", want: true},
		{name: "false", value: "false", want: false},
		{name: "false uppercase with spaces", value: " FALSE ", want: false},
		{name: "unknown value keeps cronjobs on", value: "no", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("JM_CTRD_CRON_ENABLED", tt.value)

			got := cmm.CronjobsEnabledFromEnv()
			if got != tt.want {
				t.Fatalf("CronjobsEnabledFromEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
