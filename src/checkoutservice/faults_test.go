package main

import "testing"

func TestFaultInjectorFromEnv(t *testing.T) {
	cases := map[string]float64{"": 0, "abc": 0, "-1": 0, "0.25": 0.25, "7": 1}
	for in, want := range cases {
		t.Setenv("FAULT_ERROR_RATE", in)
		if got := newFaultInjectorFromEnv().errorRate; got != want {
			t.Errorf("FAULT_ERROR_RATE=%q: rate=%v, want %v", in, got, want)
		}
	}
}

func TestFaultInjectorShouldFail(t *testing.T) {
	f := faultInjector{errorRate: 0.5, roll: func() float64 { return 0.4 }}
	if !f.shouldFail() {
		t.Fatal("roll 0.4 < rate 0.5 must fail")
	}
	f.roll = func() float64 { return 0.6 }
	if f.shouldFail() {
		t.Fatal("roll 0.6 >= rate 0.5 must pass")
	}
	if (faultInjector{errorRate: 0, roll: func() float64 { return 0 }}).shouldFail() {
		t.Fatal("rate 0 must never fail")
	}
}
