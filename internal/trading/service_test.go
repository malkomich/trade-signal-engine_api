package trading

import "testing"

func TestRoundStopPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{name: "above one", value: 123.4567, want: 123.46},
		{name: "below one", value: 0.123456, want: 0.1235},
		{name: "zero", value: 0, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := roundStopPrice(tt.value); got != tt.want {
				t.Fatalf("roundStopPrice(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestStopLossTimeInForce(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		qty  float64
		want string
	}{
		{name: "integer", qty: 10, want: "gtc"},
		{name: "fractional", qty: 2.5, want: "day"},
		{name: "zero", qty: 0, want: "gtc"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := stopLossTimeInForce(tt.qty); got != tt.want {
				t.Fatalf("stopLossTimeInForce(%v) = %q, want %q", tt.qty, got, tt.want)
			}
		})
	}
}
