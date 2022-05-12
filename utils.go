package l4la

import "time"

func timeAfter(t time.Time) <-chan time.Time {
	if t.IsZero() {
		return nil
	}

	return time.After(time.Until(t))
}
