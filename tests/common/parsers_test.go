package common_test

import (
	"testing"
	"time"

	cmm "c_trd/common"
	"c_trd/tests/testutil"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestParseTimeframeTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag  string
		want time.Duration
	}{
		{"1m", time.Minute},
		{"5M", 5 * time.Minute},
		{"15m", 15 * time.Minute},
		{"30M", 30 * time.Minute},
		{"1H", time.Hour},
		{"4h", 4 * time.Hour},
		{"1D", 24 * time.Hour},
		{"1w", 7 * 24 * time.Hour},
		// Un tag desconocido usa 1 hora por defecto
		{"3Y", time.Hour},
		{"", time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()

			if got := cmm.ParseTimeframeTag(tt.tag); got != tt.want {
				t.Errorf("ParseTimeframeTag(%q) = %v, se esperaba %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestParseHexToObjectId(t *testing.T) {
	t.Parallel()

	t.Run("hexadecimal valido", func(t *testing.T) {
		t.Parallel()

		want := primitive.NewObjectID()
		got, err := cmm.ParseHexToObjectId(want.Hex())

		testutil.RequireEmptyError(t, err)
		if got != want {
			t.Errorf("ParseHexToObjectId() = %v, se esperaba %v", got, want)
		}
	})

	t.Run("hexadecimal invalido", func(t *testing.T) {
		t.Parallel()

		_, err := cmm.ParseHexToObjectId("no-es-un-id")
		testutil.RequireTrackingCode(t, err, "CMMPE001")
	})
}
