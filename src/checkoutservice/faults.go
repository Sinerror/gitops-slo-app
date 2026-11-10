// Управляемая ошибка для учебной аварии «плохой релиз» (chaos/03-bad-release.sh).
// FAULT_ERROR_RATE=0.5 — половина PlaceOrder завершается codes.Internal,
// frontend отвечает 500 на /cart/checkout. По умолчанию 0 — поведение апстрима.
package main

import (
	"math/rand/v2"
	"os"
	"strconv"
)

type faultInjector struct {
	errorRate float64
	roll      func() float64
}

func newFaultInjectorFromEnv() faultInjector {
	rate, err := strconv.ParseFloat(os.Getenv("FAULT_ERROR_RATE"), 64)
	if err != nil || rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}
	if rate > 0 {
		log.Warnf("FAULT INJECTION ENABLED: %.0f%% of PlaceOrder calls will fail", rate*100)
	}
	return faultInjector{errorRate: rate, roll: rand.Float64}
}

func (f faultInjector) shouldFail() bool {
	return f.errorRate > 0 && f.roll() < f.errorRate
}
