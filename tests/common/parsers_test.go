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
		{"1-MINUTE", time.Minute},
		{"5-MINUTES", 5 * time.Minute},
		{"15-MINUTES", 15 * time.Minute},
		{"30-MINUTES", 30 * time.Minute},
		{"1-HOUR", time.Hour},
		{"2-HOURS", 2 * time.Hour},
		{"4-HOURS", 4 * time.Hour},
		{"1-DAY", 24 * time.Hour},
		{"1-WEEK", 7 * 24 * time.Hour},
		{"1-MONTH", 30 * 24 * time.Hour},
		// Un tag desconocido usa 1 hora por defecto
		{"4H", time.Hour},
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

func TestIsKnownTimeframeTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag  string
		want bool
	}{
		{"1-MINUTE", true},
		{"15-MINUTES", true},
		{"2-HOURS", true},
		{"1-DAY", true},
		{"1-MONTH", true},
		// Un tag desconocido no es valido aunque ParseTimeframeTag le asigne 1 hora
		{"4H", false},
		{"1-hour", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()

			if got := cmm.IsKnownTimeframeTag(tt.tag); got != tt.want {
				t.Errorf("IsKnownTimeframeTag(%q) = %v, se esperaba %v", tt.tag, got, tt.want)
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

func TestParseHexListToObjectIds(t *testing.T) {
	t.Parallel()

	first, second := primitive.NewObjectID(), primitive.NewObjectID()

	t.Run("convierte en el mismo orden", func(t *testing.T) {
		t.Parallel()

		got, err := cmm.ParseHexListToObjectIds([]string{second.Hex(), first.Hex()})
		testutil.RequireEmptyError(t, err)
		if len(got) != 2 || got[0] != second || got[1] != first {
			t.Errorf("ids = %v, se esperaba [%s %s]", got, second.Hex(), first.Hex())
		}
	})

	t.Run("lista nil retorna vacia", func(t *testing.T) {
		t.Parallel()

		got, err := cmm.ParseHexListToObjectIds(nil)
		testutil.RequireEmptyError(t, err)
		if got == nil || len(got) != 0 {
			t.Errorf("ids = %v, se esperaba []", got)
		}
	})

	t.Run("hexadecimal invalido", func(t *testing.T) {
		t.Parallel()

		_, err := cmm.ParseHexListToObjectIds([]string{first.Hex(), "abc"})
		testutil.RequireTrackingCode(t, err, "CMMPE002")
	})
}
