package model

import (
	"testing"
)

func TestToBillStatus(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    BillStatus
		wantErr bool
	}{
		{"ValidOpen", "OPEN", BillStatusOpen, false},
		{"ValidClosed", "CLOSED", BillStatusClosed, false},
		{"ValidSettled", "SETTLED", BillStatusSettled, false},
		{"InvalidStatus", "INVALID", "", true},
		{"EmptyString", "", "", true},
		{"Lowercase", "open", "", true}, // Should fail, as it expects uppercase
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToBillStatus(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToBillStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToBillStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToCurrency(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Currency
		wantErr bool
	}{
		{"ValidUSD_Uppercase", "USD", USD, false},
		{"ValidGEL_Uppercase", "GEL", GEL, false},
		{"ValidUSD_Lowercase", "usd", USD, false},
		{"ValidGEL_Lowercase", "gel", GEL, false},
		{"InvalidCurrency", "XYZ", "", true},
		{"EmptyString", "", "", true},
		{"MixedCase", "UsD", USD, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToCurrency(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToCurrency() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToCurrency() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToPolicyType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PolicyType
		wantErr bool
	}{
		{"ValidUsageBased", "USAGE_BASED", UsageBased, false},
		{"ValidMonthly", "SUBSCRIPTION", Subscription, false},
		{"InvalidType", "INVALID", "", true},
		{"EmptyString", "", "", true},
		{"Lowercase", "usage_based", "", true}, // Should fail, as it expects uppercase
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToPolicyType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToPolicyType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToPolicyType() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   string
	}{
		{"PositiveAmount", 12345, "123.45"},
		{"ZeroAmount", 0, "0.00"},
		{"NegativeAmount", -500, "-5.00"},
		{"LargeAmount", 1234567890, "12345678.90"},
		{"SmallAmount", 1, "0.01"},
		{"FractionalCent", 123, "1.23"}, // Assuming cents as smallest unit
		{"NegativeSmallAmount", -1, "-0.01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatAmount(tt.amount); got != tt.want {
				t.Errorf("FormatAmount() = %v, want %v", got, tt.want)
			}
		})
	}
}
