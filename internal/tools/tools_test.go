package tools

import (
	"context"
	"testing"
)

func TestCalculate(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		args    calcArgs
		want    float64
		wantErr bool
	}{
		{"mul", calcArgs{Op: "mul", A: 17, B: 23}, 391, false},
		{"div", calcArgs{Op: "div", A: 10, B: 4}, 2.5, false},
		{"div by zero", calcArgs{Op: "div", A: 1, B: 0}, 0, true},
		{"unknown", calcArgs{Op: "pow", A: 2, B: 3}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calculate(ctx, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("= %v, want %v", got, tt.want)
			}
		})
	}
}

// Konstruktorlar panic qilmasligini (tool ta'riflari to'g'riligini) tekshiradi.
func TestConstructorsDoNotPanic(t *testing.T) {
	if Calculator() == nil {
		t.Error("Calculator nil qaytardi")
	}
	if Clock() == nil {
		t.Error("Clock nil qaytardi")
	}
}
